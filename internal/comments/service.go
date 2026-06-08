package comments

import (
	"errors"
	"strings"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const MaxCommentLength = 280

var (
	ErrCommentNotFound = errors.New("comment not found")
	ErrPostNotFound    = errors.New("post not found")
	ErrUserNotSynced   = errors.New("user not synced; call POST /api/auth/sync")
	ErrEmptyContent    = errors.New("content is required")
	ErrContentTooLong  = errors.New("content exceeds 280 characters")
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ListByPost(postID uuid.UUID) ([]models.Comment, error) {
	var out []models.Comment
	err := s.db.
		Preload("User").
		Where("post_id = ?", postID).
		Order("created_at ASC").
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// userIDFromClerk maps a Clerk subject to our internal user UUID.
// Returns ErrUserNotSynced if the user hasn't been synced yet.
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

// Create inserts a new comment on the given post for the authenticated user.
// Returns the comment with User preloaded so the mobile client can render immediately.
func (s *Service) Create(clerkID string, postID uuid.UUID, content string) (*models.Comment, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrEmptyContent
	}
	if len(content) > MaxCommentLength {
		return nil, ErrContentTooLong
	}

	userID, err := s.userIDFromClerk(clerkID)
	if err != nil {
		return nil, err
	}

	if err := s.ensurePostExists(postID); err != nil {
		return nil, err
	}

	cm := models.Comment{
		UserID:  userID,
		PostID:  postID,
		Content: content,
	}
	if err := s.db.Create(&cm).Error; err != nil {
		return nil, err
	}

	var out models.Comment
	if err := s.db.Preload("User").First(&out, "id = ?", cm.ID).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a comment the caller owns. Single-query ownership: returns
// ErrCommentNotFound for both "doesn't exist" and "exists but not yours" —
// avoids leaking existence via a 403-vs-404 distinction.
func (s *Service) Delete(clerkID string, commentID uuid.UUID) error {
	userID, err := s.userIDFromClerk(clerkID)
	if err != nil {
		return err
	}

	res := s.db.
		Where("id = ? AND user_id = ?", commentID, userID).
		Delete(&models.Comment{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCommentNotFound
	}
	return nil
}
