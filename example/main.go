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

	client := erlcgo.NewClient(
		apiKey,
		erlcgo.WithTimeout(time.Second*15),
		erlcgo.WithRequestQueue(2, time.Millisecond*200),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	players, err := client.GetPlayers(ctx)
	if err != nil {
		if apiErr, ok := err.(*erlcgo.APIError); ok {
			log.Fatalf("API Error (%d): %s\nFriendly message: %s",
				apiErr.Code,
				apiErr.Error(),
				erlcgo.GetFriendlyErrorMessage(apiErr))
		}
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Players online: %d\n", len(players))
}
