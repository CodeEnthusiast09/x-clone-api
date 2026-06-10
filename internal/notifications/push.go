package notifications

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const expoPushURL = "https://exp.host/--/api/v2/push/send"

var pushHTTPClient = &http.Client{Timeout: 10 * time.Second}

type expoPushMessage struct {
	To    string            `json:"to"`
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
}

// SendPush fetches the recipient's push tokens and actor's display name,
// then fires a batch POST to Expo's push gateway. Intended to be called
// as a goroutine — errors are logged and never returned.
func SendPush(db *gorm.DB, recipientID, actorID uuid.UUID, nType string, postID *uuid.UUID) {
	// 1. Fetch actor display name
	var actor models.User
	if err := db.Select("first_name, last_name, username").First(&actor, "id = ?", actorID).Error; err != nil {
		log.Printf("notifications.SendPush: fetch actor: %v", err)
		return
	}
	name := strings.TrimSpace(actor.FirstName + " " + actor.LastName)
	if name == "" {
		name = actor.Username
	}

	// 2. Build human-readable title + body
	var title, body string
	switch nType {
	case "like":
		title = "New like"
		body = name + " liked your post"
	case "repost":
		title = "New repost"
		body = name + " reposted your post"
	case "comment":
		title = "New comment"
		body = name + " commented on your post"
	case "follow":
		title = "New follower"
		body = name + " followed you"
	default:
		return
	}

	// 2b. Build data payload for deep-link on tap
	data := map[string]string{
		"type":          nType,
		"actorUsername": actor.Username,
	}
	if postID != nil {
		data["postId"] = postID.String()
	}

	// 3. Fetch recipient's registered push tokens
	var tokens []models.PushToken
	if err := db.Where("user_id = ?", recipientID).Find(&tokens).Error; err != nil {
		log.Printf("notifications.SendPush: fetch tokens: %v", err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	// 4. Build message batch and POST to Expo
	messages := make([]expoPushMessage, len(tokens))
	for i, t := range tokens {
		messages[i] = expoPushMessage{To: t.Token, Title: title, Body: body, Data: data}
	}
	payload, err := json.Marshal(messages)
	if err != nil {
		log.Printf("notifications.SendPush: marshal: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, expoPushURL, bytes.NewReader(payload))
	if err != nil {
		log.Printf("notifications.SendPush: create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := pushHTTPClient.Do(req)
	if err != nil {
		log.Printf("notifications.SendPush: http post: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("notifications.SendPush: expo returned status %d", resp.StatusCode)
	}
}
