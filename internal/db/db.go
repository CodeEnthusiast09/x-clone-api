package db

import (
	"log"
	"time"

	"github.com/CodeEnthusiast09/x-clone-api/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(dsn string) *gorm.DB {
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open postgres connection: %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("failed to ping postgres: %v", err)
	}

	log.Println("postgres connected")
	return gormDB
}

func Migrate(gormDB *gorm.DB) {
	if err := gormDB.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto`).Error; err != nil {
		log.Fatalf("failed to enable pgcrypto extension: %v", err)
	}

	if err := gormDB.AutoMigrate(
		&models.User{},
		&models.Post{},
		&models.Comment{},
	); err != nil {
		log.Fatalf("auto-migrate failed: %v", err)
	}

	log.Println("schema migrated")
}
