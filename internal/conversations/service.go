package conversations

import (
	"errors"
	"strings"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrUserNotSynced is returned when a clerkID has no matching user row yet.
var ErrUserNotSynced = errors.New("user not synced")

// ConversationView is the enriched shape returned by ListForUser — includes the
// most recent message and the caller's unread count.
type ConversationView struct {
	models.Conversation
	LastMessage *models.Message `json:"lastMessage"`
	UnreadCount int64           `json:"unreadCount"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GetOrCreate returns the existing conversation between the caller and the
// recipient, or creates one. Participant IDs are sorted before insert so the
// unique index is always hit with the same canonical ordering.
func (s *Service) GetOrCreate(callerClerkID string, recipientID uuid.UUID) (*models.Conversation, error) {
	callerID, err := s.userIDFromClerk(callerClerkID)
	if err != nil {
		return nil, err
	}

	p1, p2 := sortedPair(callerID, recipientID)

	var conv models.Conversation
	err = s.db.
		Where("participant1_id = ? AND participant2_id = ?", p1, p2).
		Preload("Participant1").
		Preload("Participant2").
		First(&conv).Error

	if err == nil {
		return &conv, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	conv = models.Conversation{Participant1ID: p1, Participant2ID: p2}
	if err := s.db.Create(&conv).Error; err != nil {
		return nil, err
	}
	if err := s.db.Preload("Participant1").Preload("Participant2").First(&conv, conv.ID).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

// ListForUser returns all conversations the caller participates in, ordered by
// most recently updated, enriched with the last message and unread count.
func (s *Service) ListForUser(callerClerkID string) ([]ConversationView, error) {
	userID, err := s.userIDFromClerk(callerClerkID)
	if err != nil {
		return nil, err
	}

	var convs []models.Conversation
	if err := s.db.
		Where("participant1_id = ? OR participant2_id = ?", userID, userID).
		Preload("Participant1").
		Preload("Participant2").
		Order("updated_at DESC").
		Find(&convs).Error; err != nil {
		return nil, err
	}

	views := make([]ConversationView, 0, len(convs))
	for _, conv := range convs {
		view := ConversationView{Conversation: conv}

		var last models.Message
		if err := s.db.
			Where("conversation_id = ?", conv.ID).
			Preload("Sender").
			Order("created_at DESC").
			Limit(1).
			First(&last).Error; err == nil {
			view.LastMessage = &last
		}

		s.db.Model(&models.Message{}).
			Where("conversation_id = ? AND sender_id != ? AND read_at IS NULL", conv.ID, userID).
			Count(&view.UnreadCount)

		views = append(views, view)
	}
	return views, nil
}

// GetByID fetches a single conversation, verifying the caller is a participant.
// Returns nil (no error) when not found or the caller is not a participant.
func (s *Service) GetByID(id uuid.UUID, callerClerkID string) (*models.Conversation, error) {
	callerID, err := s.userIDFromClerk(callerClerkID)
	if err != nil {
		return nil, err
	}

	var conv models.Conversation
	err = s.db.
		Where("id = ? AND (participant1_id = ? OR participant2_id = ?)", id, callerID, callerID).
		Preload("Participant1").
		Preload("Participant2").
		First(&conv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &conv, err
}

func (s *Service) userIDFromClerk(clerkID string) (uuid.UUID, error) {
	var u models.User
	err := s.db.Select("id").Where("clerk_id = ?", clerkID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, ErrUserNotSynced
	}
	return u.ID, err
}

// sortedPair returns the two UUIDs in lexicographic order so the composite
// unique index always receives a canonical (low, high) pair.
func sortedPair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if strings.Compare(a.String(), b.String()) <= 0 {
		return a, b
	}
	return b, a
}
