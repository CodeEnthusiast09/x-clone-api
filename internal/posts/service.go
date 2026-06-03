package posts

import (
	"errors"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPostNotFound = errors.New("post not found")
	ErrUserNotFound = errors.New("user not found")
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(page, limit int) ([]models.Post, int64, error) {
	var (
		out   []models.Post
		total int64
	)

	if err := s.db.Model(&models.Post{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := s.db.
		Preload("User").
		Order("created_at DESC").
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

func (s *Service) ListByUsername(username string, page, limit int) ([]models.Post, int64, error) {
	var user models.User
	err := s.db.Select("id").Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, ErrUserNotFound
	}
	if err != nil {
		return nil, 0, err
	}

	var (
		out   []models.Post
		total int64
	)

	if err := s.db.Model(&models.Post{}).Where("user_id = ?", user.ID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err = s.db.
		Preload("User").
		Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&out).Error
	if err != nil {
		return nil, 0, err
	}

	return out, total, nil
}
