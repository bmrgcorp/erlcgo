package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bmrgcorp/erlcgo"
)

func main() {
	apiKey := os.Getenv("ERLC_API_KEY")
	if apiKey == "" {
		log.Fatal("ERLC_API_KEY environment variable is required")
	}

	cacheConfig := &erlcgo.CacheConfig{
		Enabled:      true,
		StaleIfError: true,
		TTL:          time.Second * 1,
		Cache:        erlcgo.NewMemoryCache(),
	}

	client := erlcgo.NewClient(
		apiKey,
		erlcgo.WithTimeout(time.Second*15),
		erlcgo.WithRequestQueue(2, time.Millisecond*200),
		erlcgo.WithCache(cacheConfig),
	)

	ctx := context.Background()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		start := time.Now()
		players, err := client.GetPlayers(ctx)
		duration := time.Since(start)

		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Printf("Players online: %d (took %v)\n", len(players), duration)
	}
}
