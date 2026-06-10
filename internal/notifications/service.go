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
		return
	}
	go SendPush(db, recipientID, actorID, nType, postID)
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

func (s *Service) UnreadCount(recipientID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.Model(&models.Notification{}).
		Where("recipient_id = ? AND read = false", recipientID).
		Count(&count).Error
	return count, err
}

func (s *Service) MarkAllRead(recipientID uuid.UUID) error {
	return s.db.Model(&models.Notification{}).
		Where("recipient_id = ? AND read = false", recipientID).
		Update("read", true).Error
}

// UpsertPushToken registers a device token for a user.
// ON CONFLICT updates the user_id so a reassigned device (logout + new login)
// always points to the current user.
func (s *Service) UpsertPushToken(userID uuid.UUID, token string) error {
	return s.db.Exec(
		`INSERT INTO push_tokens (id, user_id, token, created_at, updated_at)
		 VALUES (gen_random_uuid(), ?, ?, NOW(), NOW())
		 ON CONFLICT (user_id, token) DO UPDATE SET updated_at = NOW()`,
		userID, token,
	).Error
}

func (s *Service) DeletePushToken(userID uuid.UUID, token string) error {
	return s.db.Where("user_id = ? AND token = ?", userID, token).
		Delete(&models.PushToken{}).Error
}
