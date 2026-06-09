package models

import (
	"time"

	"github.com/google/uuid"
)

type PushToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"          json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_push_user_token" json:"userId"`
	Token     string    `gorm:"not null;uniqueIndex:idx_push_user_token"                json:"token"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
