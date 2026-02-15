package relay

import (
	"context"
	"fmt"
	"sync"
	"time"

	"insolventbydesign/internal/model"
)

// ParallelFetcher fetches relay data concurrently with configurable worker pools.
// Designed for high-throughput data collection (100K+ slots).
type ParallelFetcher struct {
	client      *Client
	workerCount int
	rateLimiter <-chan time.Time
}

// FetchConfig configures parallel fetching behavior.
type FetchConfig struct {
	WorkerCount    int           // Number of concurrent workers
	RateLimit      time.Duration // Minimum time between requests per worker
	RetryAttempts  int           // Number of retries on failure
	RetryBackoff   time.Duration // Backoff between retries
	ProgressReport int           // Report progress every N slots (0 = no reporting)
}

// DefaultFetchConfig returns production-grade defaults.
func DefaultFetchConfig() FetchConfig {
	return FetchConfig{
		WorkerCount:    50,                    // High concurrency
		RateLimit:      20 * time.Millisecond, // 50 RPS per worker = 2500 RPS total
		RetryAttempts:  3,
		RetryBackoff:   time.Second,
		ProgressReport: 10000,
	}
}

// NewParallelFetcher creates a fetcher with custom configuration.
func NewParallelFetcher(client *Client, config FetchConfig) *ParallelFetcher {
	return &ParallelFetcher{
		client:      client,
		workerCount: config.WorkerCount,
		rateLimiter: time.Tick(config.RateLimit),
	}
}

// SlotRange represents a range of slots to fetch.
type SlotRange struct {
	Start uint64
	End   uint64
}

// FetchResult contains fetched data and metadata.
type FetchResult struct {
	Bribes        []model.SlotBribe
	TotalFetched  uint64
	FailedSlots   []uint64
	Duration      time.Duration
	ThroughputRPS float64
}

// FetchSlotsParallel fetches relay data for a slot range using worker pool.
// Returns comprehensive results including performance metrics.
func (f *ParallelFetcher) FetchSlotsParallel(ctx context.Context, slotRange SlotRange, config FetchConfig) (*FetchResult, error) {
	startTime := time.Now()
	totalSlots := slotRange.End - slotRange.Start + 1

	// Create work queue
	slotQueue := make(chan uint64, totalSlots)
	for slot := slotRange.Start; slot <= slotRange.End; slot++ {
		slotQueue <- slot
	}
	close(slotQueue)

	// Result collection
	results := make(chan model.SlotBribe, totalSlots)
	errors := make(chan uint64, totalSlots)

	// Progress tracking
	var progressMu sync.Mutex
	var processed uint64

	// Worker pool
	var wg sync.WaitGroup
	for i := 0; i < config.WorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for slot := range slotQueue {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Rate limiting
				<-f.rateLimiter

				// Fetch with retry logic
				bribe, err := f.fetchWithRetry(ctx, slot, config.RetryAttempts, config.RetryBackoff)
				if err != nil {
					errors <- slot
					continue
				}

				results <- bribe

				// Progress reporting
				if config.ProgressReport > 0 {
					progressMu.Lock()
					processed++
					if processed%uint64(config.ProgressReport) == 0 {
						pct := float64(processed) / float64(totalSlots) * 100
						elapsed := time.Since(startTime)
						rps := float64(processed) / elapsed.Seconds()
						fmt.Printf("[Worker Pool] Progress: %d/%d (%.1f%%) | RPS: %.1f | Elapsed: %v\n",
							processed, totalSlots, pct, rps, elapsed.Round(time.Second))
					}
					progressMu.Unlock()
				}
			}
		}(i)
	}

	// Wait for completion
	wg.Wait()
	close(results)
	close(errors)

	// Collect results
	bribes := make([]model.SlotBribe, 0, totalSlots)
	for bribe := range results {
		bribes = append(bribes, bribe)
	}

	failedSlots := make([]uint64, 0)
	for slot := range errors {
		failedSlots = append(failedSlots, slot)
	}

	duration := time.Since(startTime)
	rps := float64(len(bribes)) / duration.Seconds()

	return &FetchResult{
		Bribes:        bribes,
		TotalFetched:  uint64(len(bribes)),
		FailedSlots:   failedSlots,
		Duration:      duration,
		ThroughputRPS: rps,
	}, nil
}

// fetchWithRetry attempts to fetch a slot with exponential backoff.
func (f *ParallelFetcher) fetchWithRetry(ctx context.Context, slot uint64, attempts int, backoff time.Duration) (model.SlotBribe, error) {
	var lastErr error

	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return model.SlotBribe{}, ctx.Err()
		default:
		}

		// Simulate fetch (replace with actual HTTP call)
		bribe, err := f.fetchSlot(ctx, slot)
		if err == nil {
			return bribe, nil
		}

		lastErr = err
		if i < attempts-1 {
			time.Sleep(backoff * time.Duration(1<<i)) // Exponential backoff
		}
	}

	return model.SlotBribe{}, fmt.Errorf("failed after %d attempts: %w", attempts, lastErr)
}

// fetchSlot performs the actual HTTP fetch (stub for now).
func (f *ParallelFetcher) fetchSlot(ctx context.Context, slot uint64) (model.SlotBribe, error) {
	// TODO: Implement actual HTTP call to relay API
	// This is a placeholder that demonstrates the interface
	return model.SlotBribe{
		Slot:          slot,
		ValueWei:      nil, // Replace with parsed data
		BuilderPubkey: "",
	}, fmt.Errorf("not implemented")
}

// BatchFetchMultipleRelays fetches from multiple relays concurrently and merges results.
func (f *ParallelFetcher) BatchFetchMultipleRelays(ctx context.Context, relayURLs []string, slotRange SlotRange, config FetchConfig) (*FetchResult, error) {
	resultsChan := make(chan *FetchResult, len(relayURLs))
	errChan := make(chan error, len(relayURLs))

	var wg sync.WaitGroup
	for _, relayURL := range relayURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			// Create client for this relay
			client := NewClient(url)
			fetcher := NewParallelFetcher(client, config)

			result, err := fetcher.FetchSlotsParallel(ctx, slotRange, config)
			if err != nil {
				errChan <- err
				return
			}

			resultsChan <- result
		}(relayURL)
	}

	wg.Wait()
	close(resultsChan)
	close(errChan)

	// Merge results from all relays
	merged := &FetchResult{
		Bribes:      make([]model.SlotBribe, 0),
		FailedSlots: make([]uint64, 0),
	}

	for result := range resultsChan {
		merged.Bribes = append(merged.Bribes, result.Bribes...)
		merged.FailedSlots = append(merged.FailedSlots, result.FailedSlots...)
		merged.TotalFetched += result.TotalFetched
	}

	return merged, nil
}
