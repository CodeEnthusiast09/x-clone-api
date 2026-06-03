package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	ClerkID  string `gorm:"type:varchar(255);uniqueIndex;not null" json:"clerkId"`
	Email    string `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Username string `gorm:"type:varchar(50);uniqueIndex;not null"  json:"username"`

	FirstName string `gorm:"type:varchar(100);not null" json:"firstName"`
	LastName  string `gorm:"type:varchar(100);not null" json:"lastName"`

	ProfilePicture string `gorm:"type:text;default:''"               json:"profilePicture"`
	BannerImage    string `gorm:"type:text;default:''"               json:"bannerImage"`
	Bio            string `gorm:"type:varchar(160);default:''"       json:"bio"`
	Location       string `gorm:"type:varchar(100);default:''"       json:"location"`

	Followers []*User `gorm:"many2many:user_followers;joinForeignKey:UserID;joinReferences:FollowerID" json:"followers,omitempty"`
	Following []*User `gorm:"many2many:user_followers;joinForeignKey:FollowerID;joinReferences:UserID" json:"following,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (User) TableName() string { return "users" }
