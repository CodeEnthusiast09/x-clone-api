package notifications

import (
	"errors"
	"log"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrUserNotSynced = errors.New("user not synced; call POST /api/auth/sync")

// Create is a fire-and-forget helper called by posts, comments, and follows
// as a side-effect after their primary write. Skips self-notifications silently
// and logs but never propagates DB errors so a notification failure never fails
// the parent operation.
func Create(db *gorm.DB, recipientID, actorID uuid.UUID, nType string, postID *uuid.UUID) {
	if recipientID == actorID {
		return
	}
	n := models.Notification{
		RecipientID: recipientID,
		ActorID:     actorID,
		Type:        nType,
		PostID:      postID,
	}
	if err := db.Create(&n).Error; err != nil {
		log.Printf("notifications.Create: %v", err)
	}
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

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

func (s *Service) List(recipientID uuid.UUID, page, limit int) ([]models.Notification, int64, error) {
	var (
		out   []models.Notification
		total int64
	)

	if err := s.db.Model(&models.Notification{}).
		Where("recipient_id = ?", recipientID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := s.db.
		Preload("Actor").
		Where("recipient_id = ?", recipientID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&out).Error

	return out, total, err
}

func (s *Service) MarkAllRead(recipientID uuid.UUID) error {
	return s.db.Model(&models.Notification{}).
		Where("recipient_id = ? AND read = false", recipientID).
		Update("read", true).Error
}
