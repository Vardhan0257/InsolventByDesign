package main

import (
	"log"
	"os"

	"insolventbydesign/internal/relay"
)

func main() {
	relays := []string{
		"https://boost-relay.flashbots.net",
		"https://relay.ultrasound.money",
	}

	outDir := "data/relay_raw"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatal(err)
	}

	for _, url := range relays {
		log.Printf("Fetching from %s\n", url)
		if err := relay.FetchAndStore(url, outDir); err != nil {
			log.Println("error:", err)
		}
	}
}
