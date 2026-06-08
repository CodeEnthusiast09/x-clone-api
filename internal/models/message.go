package models

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	ConversationID uuid.UUID     `gorm:"type:uuid;not null;index"                              json:"conversationId"`
	Conversation   *Conversation `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE" json:"conversation,omitempty"`

	SenderID uuid.UUID `gorm:"type:uuid;not null;index" json:"senderId"`
	Sender   *User     `gorm:"foreignKey:SenderID"      json:"sender,omitempty"`

	Body string `gorm:"type:text;not null" json:"body"`

	// nil = unread
	ReadAt *time.Time `gorm:"default:null" json:"readAt"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Message) TableName() string { return "messages" }
