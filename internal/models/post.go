package models

import (
	"time"

	"github.com/google/uuid"
)

type Post struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	UserID uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	User   *User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`

	Content string `gorm:"type:varchar(280);default:''" json:"content"`
	Image   string `gorm:"type:text;default:''"         json:"image"`

	Likes    []*User   `gorm:"many2many:post_likes;"                          json:"likes,omitempty"`
	Reposts  []*User   `gorm:"many2many:post_reposts;"                          json:"reposts,omitempty"`
	Comments []Comment `gorm:"foreignKey:PostID;constraint:OnDelete:CASCADE"  json:"comments,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Post) TableName() string { return "posts" }
