package comments

import (
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
