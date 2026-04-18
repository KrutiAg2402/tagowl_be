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

	"tagowl/backend/internal/catalog"
)

func main() {
	port := getEnv("PORT", "8080")
	mongoURI := getEnv("MONGODB_URI", "mongodb://localhost:27017/")
	mongoDatabase := getEnv("MONGODB_DATABASE", "tag_owl")
	mongoCollection := getEnv("MONGODB_COLLECTION", "producer")
	seedFile := getEnv("STICKER_SEED_FILE", "data/stickers.json")

	repo, err := catalog.NewMongoRepository(mongoURI, mongoDatabase, mongoCollection, seedFile)
	if err != nil {
		log.Fatalf("load sticker catalog: %v", err)
	}
	defer repo.Close()

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           catalog.NewHandler(repo),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	log.Printf("sticker API listening on http://localhost:%s", port)
	log.Printf("using mongodb uri %s", mongoURI)
	log.Printf("using mongodb database %s", mongoDatabase)
	log.Printf("using mongodb collection %s", mongoCollection)
	log.Printf("using seed file %s", seedFile)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
