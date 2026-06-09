package posts

import (
	"errors"
	"strings"

	"github.com/CodeEnthusiast09/x-clone-api/internal/cloudinary"
	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/CodeEnthusiast09/x-clone-api/internal/notifications"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostImageNamespace is the public_id prefix under which all post-image uploads
// must be stored. Combined with the uploader's clerkID, it forms an owner-scoped
// path (e.g. "x_clone/posts/users/user_abc/<uuid>") that we validate on Create
// (reject foreign URLs) and on Delete (gate the Cloudinary destroy call).
const PostImageNamespace = "x_clone/posts/users"

var (
	ErrPostNotFound    = errors.New("post not found")
	ErrUserNotFound    = errors.New("user not found")
	ErrUserNotSynced   = errors.New("user not synced; call POST /api/auth/sync")
	ErrEmptyPost       = errors.New("post must have content or image")
	ErrInvalidImageURL = errors.New("image URL must be a Cloudinary asset uploaded by the caller")
)

// expectedImagePrefix returns the public_id prefix that identifies images
// uploaded by the given clerkID. Used by both Create validation and the
// pre-destroy check on Delete.
func expectedImagePrefix(clerkID string) string {
	return PostImageNamespace + "/" + clerkID + "/"
}

// PostView is the feed-list shape: the full Post plus pre-computed counts and
// a per-caller liked flag so the client can render interactive like buttons.
type PostView struct {
	models.Post
	LikesCount      int64 `json:"likesCount"`
	CommentsCount   int64 `json:"commentsCount"`
	IsLikedByCaller bool  `json:"isLikedByCurrentUser"`
}

// feedCols builds the SELECT clause for feed queries.
// When callerID is not nil the clause includes a correlated EXISTS subquery
// that tells the client whether the caller has already liked each post.
func feedCols(callerID uuid.UUID) (string, []any) {
	base := `posts.*,
		(SELECT COUNT(*) FROM post_likes WHERE post_id = posts.id) AS likes_count,
		(SELECT COUNT(*) FROM comments  WHERE post_id = posts.id) AS comments_count`
	if callerID != uuid.Nil {
		return base + `,
		EXISTS(SELECT 1 FROM post_likes WHERE post_id = posts.id AND user_id = ?) AS is_liked_by_caller`,
			[]any{callerID}
	}
	return base + `, false AS is_liked_by_caller`, nil
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(callerID uuid.UUID, page, limit int) ([]PostView, int64, error) {
	var (
		out   []PostView
		total int64
	)

	if err := s.db.Model(&models.Post{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	cols, args := feedCols(callerID)
	offset := (page - 1) * limit
	err := s.db.Model(&models.Post{}).
		Select(cols, args...).
		Preload("User").
		Order("posts.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&out).Error
	if err != nil {
		return nil, 0, err
	}

	return out, total, nil
}

func (s *Service) GetByID(id uuid.UUID) (*models.Post, error) {
	var p models.Post
	err := s.db.
		Preload("User").
		Preload("Comments").
		Preload("Comments.User").
		First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPostNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// userIDFromClerk maps a Clerk subject to our internal user UUID.
// Returns ErrUserNotSynced if the user hasn't been synced (webhook missed + /sync not called).
func (s *Service) userIDFromClerk(clerkID string) (uuid.UUID, error) {
	var u models.User
	err := s.db.Select("id").Where("clerk_id = ?", clerkID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, ErrUserNotSynced
	}
	if err != nil {
		return uuid.Nil, err
	}
	return u.ID, nil
}

// Create inserts a new post for the authenticated user.
// Returns the post with User preloaded so the mobile client can render immediately.
//
// If image is non-empty, its public_id must start with the caller's owner-scoped
// prefix (PostImageNamespace + "/" + clerkID + "/"). Foreign URLs are rejected with
// ErrInvalidImageURL — this prevents one user from "claiming" another user's
// Cloudinary asset and then deleting it (the asset would still be theirs to destroy
// since Cloudinary identifies assets by public_id, not by the owning post row).
func (s *Service) Create(clerkID, content, image string) (*models.Post, error) {
	content = strings.TrimSpace(content)
	if content == "" && image == "" {
		return nil, ErrEmptyPost
	}

	if image != "" {
		publicID := cloudinary.PublicIDFromURL(image)
		if publicID == "" || !strings.HasPrefix(publicID, expectedImagePrefix(clerkID)) {
			return nil, ErrInvalidImageURL
		}
	}

	userID, err := s.userIDFromClerk(clerkID)
	if err != nil {
		return nil, err
	}

	p := models.Post{
		UserID:  userID,
		Content: content,
		Image:   image,
	}
	if err := s.db.Create(&p).Error; err != nil {
		return nil, err
	}
	return s.GetByID(p.ID)
}

// Delete removes a post the caller owns. Single-query ownership: returns ErrPostNotFound
// for both "doesn't exist" and "exists but not owned by caller" — avoids leaking existence
// via a 403-vs-404 distinction. Returns the deleted post's image URL (empty if none) so
// the caller can clean up the Cloudinary asset.
func (s *Service) Delete(clerkID string, postID uuid.UUID) (string, error) {
	userID, err := s.userIDFromClerk(clerkID)
	if err != nil {
		return "", err
	}

	var deleted models.Post
	result := s.db.
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "image"}}}).
		Where("id = ? AND user_id = ?", postID, userID).
		Delete(&deleted)

	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", ErrPostNotFound
	}
	return deleted.Image, nil
}

// Like adds the caller to a post's likers. Idempotent: re-liking is a no-op.
// Returns ErrPostNotFound if the post doesn't exist.
func (s *Service) Like(clerkID string, postID uuid.UUID) error {
	userID, err := s.userIDFromClerk(clerkID)
	if err != nil {
		return err
	}

	if err := s.ensurePostExists(postID); err != nil {
		return err
	}

	if err := s.db.Exec(
		"INSERT INTO post_likes (post_id, user_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
		postID, userID,
	).Error; err != nil {
		return err
	}

	var post models.Post
	if err := s.db.Select("user_id").First(&post, "id = ?", postID).Error; err == nil {
		notifications.Create(s.db, post.UserID, userID, "like", &postID)
	}
	return nil
}

// Unlike removes the caller from a post's likers. Idempotent: unliking an unliked post is a no-op.
// Returns ErrPostNotFound if the post itself doesn't exist.
func (s *Service) Unlike(clerkID string, postID uuid.UUID) error {
	userID, err := s.userIDFromClerk(clerkID)
	if err != nil {
		return err
	}

	if err := s.ensurePostExists(postID); err != nil {
		return err
	}

	return s.db.Exec(
		"DELETE FROM post_likes WHERE post_id = ? AND user_id = ?",
		postID, userID,
	).Error
}

func (s *Service) ensurePostExists(postID uuid.UUID) error {
	var count int64
	if err := s.db.Model(&models.Post{}).Where("id = ?", postID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrPostNotFound
	}
	return nil
}

func (s *Service) ListByUsername(callerID uuid.UUID, username string, page, limit int) ([]PostView, int64, error) {
	var user models.User
	err := s.db.Select("id").Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, ErrUserNotFound
	}
	if err != nil {
		return nil, 0, err
	}

	var (
		out   []PostView
		total int64
	)

	if err = s.db.Model(&models.Post{}).Where("user_id = ?", user.ID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	cols, args := feedCols(callerID)
	offset := (page - 1) * limit
	err = s.db.Model(&models.Post{}).
		Select(cols, args...).
		Preload("User").
		Where("posts.user_id = ?", user.ID).
		Order("posts.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&out).Error
	if err != nil {
		return nil, 0, err
	}

	return out, total, nil
}
