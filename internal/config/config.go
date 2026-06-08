package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string
	Env  string

	DatabaseURL string

	ClerkSecretKey      string
	ClerkPublishableKey string
	ClerkWebhookSecret  string

	CloudinaryCloudName    string
	CloudinaryAPIKey       string
	CloudinaryAPISecret    string
	CloudinaryUploadPreset string

	PostImageMaxBytes   int64
	BannerImageMaxBytes int64

	ArcjetKey       string
	ArcjetPublicRPM int
	ArcjetAuthRPM   int
	ArcjetWriteRPM  int
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from process environment")
	}

	cfg := &Config{
		Port:                   getEnv("PORT", "8080"),
		Env:                    getEnv("ENV", "development"),
		DatabaseURL:            mustGet("DATABASE_URL"),
		ClerkSecretKey:         mustGet("CLERK_SECRET_KEY"),
		ClerkPublishableKey:    os.Getenv("CLERK_PUBLISHABLE_KEY"),
		ClerkWebhookSecret:     mustGet("CLERK_WEBHOOK_SECRET"),
		CloudinaryCloudName:    mustGet("CLOUDINARY_CLOUD_NAME"),
		CloudinaryAPIKey:       mustGet("CLOUDINARY_API_KEY"),
		CloudinaryAPISecret:    mustGet("CLOUDINARY_API_SECRET"),
		CloudinaryUploadPreset: mustGet("CLOUDINARY_UPLOAD_PRESET"),
		PostImageMaxBytes:      mustGetInt64("POST_IMAGE_MAX_BYTES"),
		BannerImageMaxBytes:    mustGetInt64("BANNER_IMAGE_MAX_BYTES"),
		ArcjetKey:              mustGet("ARCJET_KEY"),
		ArcjetPublicRPM:        mustGetInt("ARCJET_PUBLIC_RPM"),
		ArcjetAuthRPM:          mustGetInt("ARCJET_AUTH_RPM"),
		ArcjetWriteRPM:         mustGetInt("ARCJET_WRITE_RPM"),
	}

	return cfg
}

func mustGetInt64(key string) int64 {
	raw := mustGet(key)
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		log.Fatalf("invalid int64 value for %s: %v", key, err)
	}
	if n <= 0 {
		log.Fatalf("env var %s must be a positive int64, got %d", key, n)
	}
	return n
}

func mustGetInt(key string) int {
	raw := mustGet(key)
	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Fatalf("invalid int value for %s: %v", key, err)
	}
	if n <= 0 {
		log.Fatalf("env var %s must be a positive int, got %d", key, n)
	}
	return n
}

func mustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
