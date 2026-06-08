package messages

import (
	"errors"
	"time"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Create inserts a new message and returns it with the Sender preloaded.
// senderID is a DB UUID (supplied by the WebSocket handler after resolving clerkID).
func (s *Service) Create(conversationID, senderID uuid.UUID, body string) (*models.Message, error) {
	msg := models.Message{
		ConversationID: conversationID,
		SenderID:       senderID,
		Body:           body,
	}
	if err := s.db.Create(&msg).Error; err != nil {
		return nil, err
	}
	if err := s.db.Preload("Sender").First(&msg, msg.ID).Error; err != nil {
		return nil, err
	}
	// Bump conversation.updated_at so ListForUser ordering stays current.
	s.db.Model(&models.Conversation{}).Where("id = ?", conversationID).Update("updated_at", time.Now())
	return &msg, nil
}

// ListForConversation returns paginated messages in chronological order.
func (s *Service) ListForConversation(conversationID uuid.UUID, page, limit int) ([]models.Message, int64, error) {
	var total int64
	s.db.Model(&models.Message{}).Where("conversation_id = ?", conversationID).Count(&total)

	var msgs []models.Message
	offset := (page - 1) * limit
	err := s.db.
		Where("conversation_id = ?", conversationID).
		Preload("Sender").
		Order("created_at ASC").
		Offset(offset).Limit(limit).
		Find(&msgs).Error
	return msgs, total, err
}

// MarkRead sets read_at = now() for all unread messages in a conversation that
// were NOT sent by the caller. Returns the number of rows updated.
func (s *Service) MarkRead(conversationID uuid.UUID, callerClerkID string) (int64, error) {
	callerID, err := s.userIDFromClerk(callerClerkID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	result := s.db.Model(&models.Message{}).
		Where("conversation_id = ? AND sender_id != ? AND read_at IS NULL", conversationID, callerID).
		Update("read_at", now)
	return result.RowsAffected, result.Error
}

func (s *Service) userIDFromClerk(clerkID string) (uuid.UUID, error) {
	var u models.User
	err := s.db.Select("id").Where("clerk_id = ?", clerkID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, errors.New("user not synced")
	}
	return u.ID, err
}
