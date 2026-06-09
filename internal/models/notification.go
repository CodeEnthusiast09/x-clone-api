package models

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RecipientID uuid.UUID  `gorm:"type:uuid;not null;index"                        json:"recipientId"`
	ActorID     uuid.UUID  `gorm:"type:uuid;not null"                              json:"actorId"`
	Actor       User       `gorm:"foreignKey:ActorID"                              json:"actor"`
	Type        string     `gorm:"not null"                                        json:"type"`
	PostID      *uuid.UUID `gorm:"type:uuid"                                       json:"postId"`
	Read        bool       `gorm:"not null;default:false"                          json:"read"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}
