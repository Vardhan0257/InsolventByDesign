package model

import (
	"fmt"
	"sort"
)

// BuilderStats contains builder-level statistics for concentration analysis.
type BuilderStats struct {
	BuilderPubkey string
	BlockCount    uint64
}

// ComputeBuilderConcentration analyzes builder centralization from relay data.
//
// This computes the centralization coefficient α:
//
//	α = (blocks by top k builders) / (total blocks)
//
// Where k is specified by topK parameter.
//
// Rationale:
// - Lower effective censorship cost under builder concentration
// - Out-of-band coordination becomes cheaper
// - "Rent-a-cartel" economics
//
// Returns:
// - alpha: concentration coefficient α ∈ [0, 1]
// - builderStats: sorted list of builders by block count (descending)
// - error: if data is invalid
func ComputeBuilderConcentration(bribes []SlotBribe, topK int) (alpha float64, builderStats []BuilderStats, err error) {
	if len(bribes) == 0 {
		return 0, nil, fmt.Errorf("empty bribes slice")
	}

	if topK < 1 {
		return 0, nil, fmt.Errorf("topK must be at least 1, got %d", topK)
	}

	// Count blocks per builder
	builderCounts := make(map[string]uint64)
	totalBlocks := uint64(len(bribes))

	for _, bribe := range bribes {
		// Handle empty builder pubkeys
		key := bribe.BuilderPubkey
		if key == "" {
			key = "unknown"
		}
		builderCounts[key]++
	}

	// Convert to sorted slice
	stats := make([]BuilderStats, 0, len(builderCounts))
	for builder, count := range builderCounts {
		stats = append(stats, BuilderStats{
			BuilderPubkey: builder,
			BlockCount:    count,
		})
	}

	// Sort by block count descending
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].BlockCount > stats[j].BlockCount
	})

	// Compute top-k concentration
	var topKBlocks uint64
	actualK := topK
	if actualK > len(stats) {
		actualK = len(stats)
	}

	for i := 0; i < actualK; i++ {
		topKBlocks += stats[i].BlockCount
	}

	// α = top-k blocks / total blocks
	alpha = float64(topKBlocks) / float64(totalBlocks)

	return alpha, stats, nil
}

// GetTopBuilders returns the top k builders by block count.
//
// This is a convenience wrapper around ComputeBuilderConcentration
// for cases where only the builder list is needed.
func GetTopBuilders(bribes []SlotBribe, k int) ([]BuilderStats, error) {
	_, stats, err := ComputeBuilderConcentration(bribes, k)
	if err != nil {
		return nil, err
	}

	if k > len(stats) {
		k = len(stats)
	}

	return stats[:k], nil
}

// GetBuilderDiversity returns the total number of unique builders.
//
// This is a simple measure of builder diversity in the dataset.
func GetBuilderDiversity(bribes []SlotBribe) int {
	builders := make(map[string]struct{})
	for _, bribe := range bribes {
		key := bribe.BuilderPubkey
		if key == "" {
			key = "unknown"
		}
		builders[key] = struct{}{}
	}
	return len(builders)
}
