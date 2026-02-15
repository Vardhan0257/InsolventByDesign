package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client represents an HTTP client for fetching relay data.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new relay client with the specified base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type RelayBid struct {
	Slot     string `json:"slot"`
	ValueWei string `json:"value"`
}

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
