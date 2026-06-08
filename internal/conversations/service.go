package conversations

import (
	"errors"
	"strings"

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

// GetOrCreate returns the existing conversation between two users, or creates
// one. Participant IDs are sorted before insert so the unique index is always
// hit with the same ordering regardless of call direction.
func (s *Service) GetOrCreate(userA, userB uuid.UUID) (*models.Conversation, error) {
	p1, p2 := sortedPair(userA, userB)

	var conv models.Conversation
	err := s.db.
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

// ListForUser returns all conversations the user participates in, ordered by
// most recently updated. Each row is preloaded with both participants.
func (s *Service) ListForUser(userID uuid.UUID) ([]models.Conversation, error) {
	var convs []models.Conversation
	err := s.db.
		Where("participant1_id = ? OR participant2_id = ?", userID, userID).
		Preload("Participant1").
		Preload("Participant2").
		Order("updated_at DESC").
		Find(&convs).Error
	return convs, err
}

// GetByID fetches a single conversation, verifying the caller is a participant.
func (s *Service) GetByID(id, callerID uuid.UUID) (*models.Conversation, error) {
	var conv models.Conversation
	err := s.db.
		Where("id = ? AND (participant1_id = ? OR participant2_id = ?)", id, callerID, callerID).
		Preload("Participant1").
		Preload("Participant2").
		First(&conv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &conv, err
}

// sortedPair returns the two UUIDs in lexicographic order so the composite
// unique index always receives a canonical (low, high) pair.
func sortedPair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if strings.Compare(a.String(), b.String()) <= 0 {
		return a, b
	}
	return b, a
}
