package relay

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

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

	ts := time.Now().Unix()
	file := fmt.Sprintf("%s/%s_%d.json", outDir, sanitize(baseURL), ts)

	return os.WriteFile(file, body, 0644)
}

func sanitize(s string) string {
	return fmt.Sprintf("%x", s)
}
