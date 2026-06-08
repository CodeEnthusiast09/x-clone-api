package models

import (
	"time"

	"github.com/google/uuid"
)

// Conversation represents a private conversation between two users. Each conversation is uniquely
// Participant1ID always holds the lexicographically lower UUID so the pair
// (participant1_id, participant2_id) is canonical and the composite unique
// index prevents duplicate conversations between the same two users.
type Conversation struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	Participant1ID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:conversations_pair_udx" json:"participant1Id"`
	Participant1   *User     `gorm:"foreignKey:Participant1ID"                             json:"participant1,omitempty"`

	Participant2ID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:conversations_pair_udx" json:"participant2Id"`
	Participant2   *User     `gorm:"foreignKey:Participant2ID"                             json:"participant2,omitempty"`

	Messages []Message `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Conversation) TableName() string { return "conversations" }
