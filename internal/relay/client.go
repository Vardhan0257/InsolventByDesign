package relay

import (
	"fmt"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

type RelayBid struct {
	Slot      string `json:"slot"`
	ValueWei string `json:"value"`
}

/*************  ✨ Windsurf Command ⭐  *************/
// FetchAndStore retrieves the current relay bid traces from the given
// baseURL and stores the result in a JSON file in the given outDir.
// The filename of the stored file will be in the format
// `<baseURL>_<unix_timestamp>.json`.
/*******  9f6fcb3f-4c66-423b-b7b0-1c65513ec1da  *******/
func FetchAndStore(baseURL, outDir string) error {
	endpoint := fmt.Sprintf(
		"%s/relay/v1/data/bidtraces/proposer_payload_delivered",
		baseURL,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var bids []RelayBid
	if err := json.Unmarshal(body, &bids); err != nil {
		return err
	}

	ts := time.Now().Unix()
	file := fmt.Sprintf("%s/%s_%d.json", outDir, sanitize(baseURL), ts)

	return os.WriteFile(file, body, 0644)
}

func sanitize(s string) string {
	return fmt.Sprintf("%x", s)
}
