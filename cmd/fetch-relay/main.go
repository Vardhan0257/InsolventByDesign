package main

import (
	"log"
	"os"

	"insolventbydesign/internal/relay"
)

func main() {
	outDir := "data/relay_raw"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatal(err)
	}

	relays := []string{
		"https://boost-relay.flashbots.net",
		"https://relay.ultrasound.money",
	}

	for _, url := range relays {
		log.Printf("Fetching from %s\n", url)
		if err := relay.FetchAndStore(url, outDir); err != nil {
			log.Println("error:", err)
		}
	}
}
