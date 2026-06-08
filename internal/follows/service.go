package follows

import (
	"errors"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrSelfFollow    = errors.New("cannot follow yourself")
	ErrUserNotSynced = errors.New("user not synced; call POST /api/auth/sync")
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// resolveSelfAndTarget looks up the caller (via clerk_id) and the target
// (via username), returning ErrUserNotSynced if the caller's row is missing,
// ErrUserNotFound if the target doesn't exist, and ErrSelfFollow if both
// resolve to the same row.
func (s *Service) resolveSelfAndTarget(clerkID, username string) (uuid.UUID, uuid.UUID, error) {
	var me models.User
	err := s.db.Select("id").Where("clerk_id = ?", clerkID).First(&me).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, uuid.Nil, ErrUserNotSynced
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	var target models.User
	err = s.db.Select("id").Where("username = ?", username).First(&target).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, uuid.Nil, ErrUserNotFound
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	if me.ID == target.ID {
		return uuid.Nil, uuid.Nil, ErrSelfFollow
	}
	return me.ID, target.ID, nil
}

// Follow makes the caller follow the user with the given username.
// The user_followers row stores (user_id = target, follower_id = caller),
// matching the GORM many2many mapping on the User model. Idempotent:
// re-following is a no-op via ON CONFLICT DO NOTHING.
func (s *Service) Follow(clerkID, username string) error {
	meID, targetID, err := s.resolveSelfAndTarget(clerkID, username)
	if err != nil {
		return err
	}
	return s.db.Exec(
		"INSERT INTO user_followers (user_id, follower_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
		targetID, meID,
	).Error
}

// Unfollow removes the follow relationship. Idempotent: unfollowing a user
// you don't follow is a no-op (DELETE matches zero rows, no error).
func (s *Service) Unfollow(clerkID, username string) error {
	meID, targetID, err := s.resolveSelfAndTarget(clerkID, username)
	if err != nil {
		return err
	}
	return s.db.Exec(
		"DELETE FROM user_followers WHERE user_id = ? AND follower_id = ?",
		targetID, meID,
	).Error
}
