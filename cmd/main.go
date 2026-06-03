package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CodeEnthusiast09/x-clone-api/internal/config"
	"github.com/CodeEnthusiast09/x-clone-api/internal/db"
	"github.com/CodeEnthusiast09/x-clone-api/internal/router"
	"github.com/clerk/clerk-sdk-go/v2"
)

func main() {
	cfg := config.Load()
	clerk.SetKey(cfg.ClerkSecretKey)

	gormDB := db.Connect(cfg.DatabaseURL)
	db.Migrate(gormDB)

	r := router.New(cfg, gormDB)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s (env=%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err == nil {
		_ = sqlDB.Close()
	}

	log.Println("server stopped cleanly")
}
