package relay

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"

	"insolventbydesign/internal/model"
)

// RelayBidTrace represents a single delivered payload from the relay API.
// This matches the schema of /relay/v1/data/bidtraces/proposer_payload_delivered
type RelayBidTrace struct {
	Slot                 string `json:"slot"`
	ParentHash           string `json:"parent_hash"`
	BlockHash            string `json:"block_hash"`
	BuilderPubkey        string `json:"builder_pubkey"`
	ProposerPubkey       string `json:"proposer_pubkey"`
	ProposerFeeRecipient string `json:"proposer_fee_recipient"`
	GasLimit             string `json:"gas_limit"`
	GasUsed              string `json:"gas_used"`
	Value                string `json:"value"`
	NumTx                string `json:"num_tx,omitempty"`
	BlockNumber          string `json:"block_number"`
}

// ParseRelayFile loads a relay JSON file and extracts slot-level bribe data.
//
// This function is the PRIMARY data ingestion point for the entire project.
//
// Guarantees:
// - NO floating point conversion (preserves exact wei values)
// - Deterministic ordering (sorted by slot ascending)
// - Fails loudly on malformed data
// - Returns error on any inconsistency
//
// Input: path to a JSON file containing RelayBidTrace array
// Output: ordered slice of model.SlotBribe structs, or error
func ParseRelayFile(filepath string) ([]model.SlotBribe, error) {
	// Read raw file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	// Handle empty files explicitly
	if len(data) == 0 {
		return nil, fmt.Errorf("file is empty: %s", filepath)
	}

	// Parse JSON array
	var traces []RelayBidTrace
	if err := json.Unmarshal(data, &traces); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", filepath, err)
	}

	// Convert to model.SlotBribe format
	bribes := make([]model.SlotBribe, 0, len(traces))
	for i, trace := range traces {
		bribe, err := convertTraceToBribe(trace, i)
		if err != nil {
			return nil, fmt.Errorf("failed to convert trace at index %d: %w", i, err)
		}
		bribes = append(bribes, bribe)
	}

	// Sort by slot (deterministic ordering)
	sort.Slice(bribes, func(i, j int) bool {
		return bribes[i].Slot < bribes[j].Slot
	})

	return bribes, nil
}

// convertTraceToBribe extracts the minimal economic data from a relay trace.
//
// Critical conversion rules:
// - Slot: string -> uint64 (fail if not parseable)
// - Value: string -> big.Int (NO precision loss, fail if not parseable)
// - BuilderPubkey: preserved as-is for concentration analysis
func convertTraceToBribe(trace RelayBidTrace, index int) (model.SlotBribe, error) {
	// Parse slot number
	var slot uint64
	_, err := fmt.Sscanf(trace.Slot, "%d", &slot)
	if err != nil {
		return model.SlotBribe{}, fmt.Errorf("invalid slot format '%s' at index %d: %w", trace.Slot, index, err)
	}

	// Parse value as big.Int (NO floating point)
	valueWei := new(big.Int)
	_, ok := valueWei.SetString(trace.Value, 10)
	if !ok {
		return model.SlotBribe{}, fmt.Errorf("invalid value format '%s' at index %d", trace.Value, index)
	}

	// Validate non-negative
	if valueWei.Sign() < 0 {
		return model.SlotBribe{}, fmt.Errorf("negative value %s at index %d", trace.Value, index)
	}

	return model.SlotBribe{
		Slot:          slot,
		ValueWei:      valueWei,
		BuilderPubkey: trace.BuilderPubkey,
	}, nil
}

// ParseRelayDirectory loads all JSON files from a directory.
//
// This aggregates data across multiple relay snapshots.
//
// Guarantees:
// - Fails if ANY file fails to parse
// - Returns globally sorted bribes by slot
// - Deterministic output
func ParseRelayDirectory(dirpath string) ([]model.SlotBribe, error) {
	entries, err := os.ReadDir(dirpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirpath, err)
	}

	var allBribes []model.SlotBribe
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only parse .json files
		if len(entry.Name()) < 5 || entry.Name()[len(entry.Name())-5:] != ".json" {
			continue
		}

		filepath := fmt.Sprintf("%s/%s", dirpath, entry.Name())
		bribes, err := ParseRelayFile(filepath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", filepath, err)
		}

		allBribes = append(allBribes, bribes...)
	}

	// Global sort
	sort.Slice(allBribes, func(i, j int) bool {
		return allBribes[i].Slot < allBribes[j].Slot
	})

	return allBribes, nil
}
