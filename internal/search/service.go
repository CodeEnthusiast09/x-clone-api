package search

import (
	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"gorm.io/gorm"
)

type PostResult struct {
	models.Post
	LikesCount    int64 `json:"likesCount"`
	CommentsCount int64 `json:"commentsCount"`
}

type SearchResults struct {
	Users []models.User `json:"users"`
	Posts []PostResult  `json:"posts"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Search(q string, limit int) (SearchResults, error) {
	pattern := "%" + q + "%"
	var results SearchResults

	if err := s.db.
		Where("username ILIKE ? OR first_name ILIKE ? OR last_name ILIKE ?", pattern, pattern, pattern).
		Limit(limit).
		Find(&results.Users).Error; err != nil {
		return results, err
	}

	err := s.db.Model(&models.Post{}).
		Select(`posts.*,
			(SELECT COUNT(*) FROM post_likes WHERE post_id = posts.id) AS likes_count,
			(SELECT COUNT(*) FROM comments  WHERE post_id = posts.id) AS comments_count`).
		Preload("User").
		Where("content ILIKE ?", pattern).
		Order("posts.created_at DESC").
		Limit(limit).
		Find(&results.Posts).Error

	return results, err
}
