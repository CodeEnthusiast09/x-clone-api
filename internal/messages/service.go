package messages

import (
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
// were NOT sent by callerID. Returns the number of rows updated.
func (s *Service) MarkRead(conversationID, callerID uuid.UUID) (int64, error) {
	now := time.Now()
	result := s.db.Model(&models.Message{}).
		Where("conversation_id = ? AND sender_id != ? AND read_at IS NULL", conversationID, callerID).
		Update("read_at", now)
	return result.RowsAffected, result.Error
}
