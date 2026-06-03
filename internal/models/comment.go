package models

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	UserID uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	User   *User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`

	PostID uuid.UUID `gorm:"type:uuid;not null;index" json:"postId"`

	Content string `gorm:"type:varchar(280);not null" json:"content"`

	Likes []*User `gorm:"many2many:comment_likes;" json:"likes,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Comment) TableName() string { return "comments" }
