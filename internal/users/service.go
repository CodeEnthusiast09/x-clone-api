package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"github.com/clerk/clerk-sdk-go/v2"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (s *Service) GetByClerkID(clerkID string) (*models.User, error) {
	var u models.User
	err := s.db.Where("clerk_id = ?", clerkID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpsertFromClerk inserts a user keyed on clerk_id, or updates the mutable fields if it already exists.
// Used by both webhook user.created and user.updated events, and by the /sync fallback.
func (s *Service) UpsertFromClerk(clerkID, email, firstName, lastName, profilePicture string) (*models.User, error) {
	username, err := s.deriveUsername(email, clerkID)
	if err != nil {
		return nil, err
	}

	u := models.User{
		ClerkID:        clerkID,
		Email:          email,
		Username:       username,
		FirstName:      firstName,
		LastName:       lastName,
		ProfilePicture: profilePicture,
	}

	err = s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "clerk_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"email", "first_name", "last_name", "profile_picture", "updated_at",
		}),
	}).Create(&u).Error
	if err != nil {
		return nil, err
	}

	// Re-read so we get the row's id + timestamps regardless of insert vs update.
	return s.GetByClerkID(clerkID)
}

func (s *Service) DeleteByClerkID(clerkID string) error {
	res := s.db.Where("clerk_id = ?", clerkID).Delete(&models.User{})
	if res.Error != nil {
		return res.Error
	}
	// 0 rows affected is fine for delete — webhook may fire after the row is already gone.
	return nil
}

// Sync is the mobile-side fallback. If the user already exists, returns them as-is.
// If not, fetches from Clerk's API and upserts.
func (s *Service) Sync(ctx context.Context, clerkID string) (*models.User, error) {
	existing, err := s.GetByClerkID(clerkID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	cu, err := clerkuser.Get(ctx, clerkID)
	if err != nil {
		return nil, fmt.Errorf("fetch from clerk: %w", err)
	}

	email := pickPrimaryEmail(cu)
	if email == "" {
		return nil, errors.New("clerk user has no email address")
	}

	return s.UpsertFromClerk(
		clerkID,
		email,
		derefString(cu.FirstName),
		derefString(cu.LastName),
		derefString(cu.ImageURL),
	)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// deriveUsername builds a username from the email local-part. Appends a slug from clerk_id on collision.
func (s *Service) deriveUsername(email, clerkID string) (string, error) {
	base := strings.ToLower(strings.SplitN(email, "@", 2)[0])
	base = sanitizeUsername(base)
	if base == "" {
		base = "user"
	}

	// Cap at 40 so we have room for the suffix and stay under the 50-char column limit.
	if len(base) > 40 {
		base = base[:40]
	}

	// First, check if our own row already has it (idempotent retry from same user).
	var existing models.User
	err := s.db.Where("username = ?", base).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return base, nil
	}
	if err != nil {
		return "", err
	}
	if existing.ClerkID == clerkID {
		return base, nil
	}

	suffix := clerkID
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	return base + "_" + suffix, nil
}

func sanitizeUsername(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '_':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r == '.' || r == '-':
			b.WriteRune('_')
		}
	}
	return b.String()
}

func pickPrimaryEmail(cu *clerk.User) string {
	if cu == nil {
		return ""
	}
	for _, e := range cu.EmailAddresses {
		if cu.PrimaryEmailAddressID != nil && e.ID == *cu.PrimaryEmailAddressID {
			return strings.ToLower(e.EmailAddress)
		}
	}
	if len(cu.EmailAddresses) > 0 {
		return strings.ToLower(cu.EmailAddresses[0].EmailAddress)
	}
	return ""
}
