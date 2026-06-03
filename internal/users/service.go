package users

import (
	"errors"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetByUsername(username string) (*models.User, error) {
	var u models.User
	err := s.db.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
