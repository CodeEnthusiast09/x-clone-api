package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string
	Env  string

	DatabaseURL string

	ClerkSecretKey      string
	ClerkPublishableKey string
	ClerkWebhookSecret  string

	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string

	ArcjetKey string
	ArcjetEnv string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from process environment")
	}

	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		Env:                 getEnv("ENV", "development"),
		DatabaseURL:         mustGet("DATABASE_URL"),
		ClerkSecretKey:      mustGet("CLERK_SECRET_KEY"),
		ClerkPublishableKey: os.Getenv("CLERK_PUBLISHABLE_KEY"),
		ClerkWebhookSecret:  mustGet("CLERK_WEBHOOK_SECRET"),
		CloudinaryCloudName: mustGet("CLOUDINARY_CLOUD_NAME"),
		CloudinaryAPIKey:    mustGet("CLOUDINARY_API_KEY"),
		CloudinaryAPISecret: mustGet("CLOUDINARY_API_SECRET"),
		ArcjetKey:           mustGet("ARCJET_KEY"),
		ArcjetEnv:           getEnv("ARCJET_ENV", "development"),
	}

	return cfg
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
