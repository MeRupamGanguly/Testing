package main

import (
	"cloudstorage/storage"
	"context"
	"log"
)

func main() {
	ctx := context.Background()
	provider, err := storage.NewStorage(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize storage provider: %v", err)
	}
	handler := storage.NewStorageHandler(provider)
	r := storage.NewRoutes(handler)

	log.Println("Storage API running on :8080")
	r.Run(":8080")
}
