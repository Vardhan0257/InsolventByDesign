package model

import (
	"math/big"
	"testing"
)

// TestComputeBuilderConcentration_Basic verifies correct α computation.
//
// Purpose: Frequency calculation - REQUIRED by PHASE 3
func TestComputeBuilderConcentration_Basic(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xbuilder1"},
		{Slot: 2, ValueWei: big.NewInt(200), BuilderPubkey: "0xbuilder1"},
		{Slot: 3, ValueWei: big.NewInt(300), BuilderPubkey: "0xbuilder2"},
		{Slot: 4, ValueWei: big.NewInt(400), BuilderPubkey: "0xbuilder3"},
	}

	// Top 1 builder: builder1 has 2/4 = 0.5
	alpha, stats, err := ComputeBuilderConcentration(bribes, 1)
	if err != nil {
		t.Fatalf("ComputeBuilderConcentration failed: %v", err)
	}

	expectedAlpha := 0.5
	if alpha != expectedAlpha {
		t.Errorf("expected alpha=%f, got %f", expectedAlpha, alpha)
	}

	// Verify top builder is builder1
	if len(stats) < 1 {
		t.Fatal("expected at least 1 builder in stats")
	}
	if stats[0].BuilderPubkey != "0xbuilder1" {
		t.Errorf("expected top builder to be 0xbuilder1, got %s", stats[0].BuilderPubkey)
	}
	if stats[0].BlockCount != 2 {
		t.Errorf("expected builder1 to have 2 blocks, got %d", stats[0].BlockCount)
	}
}

// TestComputeBuilderConcentration_TopK verifies correct top-k selection.
//
// Purpose: Builder count / identity correctness - REQUIRED by PHASE 3
func TestComputeBuilderConcentration_TopK(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(200), BuilderPubkey: "0xA"},
		{Slot: 3, ValueWei: big.NewInt(300), BuilderPubkey: "0xA"},
		{Slot: 4, ValueWei: big.NewInt(400), BuilderPubkey: "0xB"},
		{Slot: 5, ValueWei: big.NewInt(500), BuilderPubkey: "0xB"},
		{Slot: 6, ValueWei: big.NewInt(600), BuilderPubkey: "0xC"},
		{Slot: 7, ValueWei: big.NewInt(700), BuilderPubkey: "0xD"},
		{Slot: 8, ValueWei: big.NewInt(800), BuilderPubkey: "0xE"},
	}

	// 8 total blocks
	// 0xA: 3, 0xB: 2, 0xC: 1, 0xD: 1, 0xE: 1

	// Top 2: A + B = 5/8 = 0.625
	alpha, stats, err := ComputeBuilderConcentration(bribes, 2)
	if err != nil {
		t.Fatalf("ComputeBuilderConcentration failed: %v", err)
	}

	expectedAlpha := 5.0 / 8.0
	if alpha != expectedAlpha {
		t.Errorf("expected alpha=%f, got %f", expectedAlpha, alpha)
	}

	// Verify ordering
	if len(stats) != 5 {
		t.Errorf("expected 5 unique builders, got %d", len(stats))
	}
	if stats[0].BuilderPubkey != "0xA" || stats[0].BlockCount != 3 {
		t.Errorf("expected top builder 0xA with 3 blocks, got %s with %d", stats[0].BuilderPubkey, stats[0].BlockCount)
	}
	if stats[1].BuilderPubkey != "0xB" || stats[1].BlockCount != 2 {
		t.Errorf("expected 2nd builder 0xB with 2 blocks, got %s with %d", stats[1].BuilderPubkey, stats[1].BlockCount)
	}
}

// TestComputeBuilderConcentration_SingleBuilder verifies edge case.
//
// Purpose: Edge case (single builder) - REQUIRED by PHASE 3
func TestComputeBuilderConcentration_SingleBuilder(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xmonopoly"},
		{Slot: 2, ValueWei: big.NewInt(200), BuilderPubkey: "0xmonopoly"},
		{Slot: 3, ValueWei: big.NewInt(300), BuilderPubkey: "0xmonopoly"},
	}

	// Single builder = α should be 1.0 for any k >= 1
	alpha, stats, err := ComputeBuilderConcentration(bribes, 1)
	if err != nil {
		t.Fatalf("ComputeBuilderConcentration failed: %v", err)
	}

	if alpha != 1.0 {
		t.Errorf("expected alpha=1.0 for single builder, got %f", alpha)
	}

	if len(stats) != 1 {
		t.Errorf("expected 1 builder in stats, got %d", len(stats))
	}
}

// TestComputeBuilderConcentration_EmptyData verifies failure on empty input.
func TestComputeBuilderConcentration_EmptyData(t *testing.T) {
	bribes := []SlotBribe{}

	_, _, err := ComputeBuilderConcentration(bribes, 1)
	if err == nil {
		t.Error("Expected error for empty bribes, got nil")
	}
}

// TestComputeBuilderConcentration_InvalidK verifies failure on invalid k.
func TestComputeBuilderConcentration_InvalidK(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xbuilder"},
	}

	_, _, err := ComputeBuilderConcentration(bribes, 0)
	if err == nil {
		t.Error("Expected error for topK=0, got nil")
	}

	_, _, err = ComputeBuilderConcentration(bribes, -1)
	if err == nil {
		t.Error("Expected error for topK=-1, got nil")
	}
}

// TestComputeBuilderConcentration_KExceedsBuilders verifies handling when k > builders.
func TestComputeBuilderConcentration_KExceedsBuilders(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(200), BuilderPubkey: "0xB"},
	}

	// Only 2 builders, but ask for top 10
	alpha, stats, err := ComputeBuilderConcentration(bribes, 10)
	if err != nil {
		t.Fatalf("ComputeBuilderConcentration failed: %v", err)
	}

	// Should include all builders, so α = 1.0
	if alpha != 1.0 {
		t.Errorf("expected alpha=1.0 when k exceeds builder count, got %f", alpha)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 builders in stats, got %d", len(stats))
	}
}

// TestComputeBuilderConcentration_EmptyPubkey verifies handling of empty builder IDs.
func TestComputeBuilderConcentration_EmptyPubkey(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: ""},
		{Slot: 2, ValueWei: big.NewInt(200), BuilderPubkey: ""},
		{Slot: 3, ValueWei: big.NewInt(300), BuilderPubkey: "0xbuilder"},
	}

	// Empty pubkeys should be grouped as "unknown"
	alpha, stats, err := ComputeBuilderConcentration(bribes, 1)
	if err != nil {
		t.Fatalf("ComputeBuilderConcentration failed: %v", err)
	}

	// Top builder should be "unknown" with 2 blocks
	if stats[0].BuilderPubkey != "unknown" {
		t.Errorf("expected top builder to be 'unknown', got %s", stats[0].BuilderPubkey)
	}
	if stats[0].BlockCount != 2 {
		t.Errorf("expected 'unknown' to have 2 blocks, got %d", stats[0].BlockCount)
	}

	// α = 2/3
	expectedAlpha := 2.0 / 3.0
	if alpha != expectedAlpha {
		t.Errorf("expected alpha=%f, got %f", expectedAlpha, alpha)
	}
}

// TestComputeBuilderConcentration_Distribution verifies statistical correctness.
func TestComputeBuilderConcentration_Distribution(t *testing.T) {
	// Create a more realistic distribution
	bribes := []SlotBribe{}

	// Builder A: 50 blocks
	for i := 0; i < 50; i++ {
		bribes = append(bribes, SlotBribe{
			Slot:          uint64(i),
			ValueWei:      big.NewInt(int64(i)),
			BuilderPubkey: "0xA",
		})
	}

	// Builder B: 30 blocks
	for i := 0; i < 30; i++ {
		bribes = append(bribes, SlotBribe{
			Slot:          uint64(i + 50),
			ValueWei:      big.NewInt(int64(i)),
			BuilderPubkey: "0xB",
		})
	}

	// Builder C: 20 blocks
	for i := 0; i < 20; i++ {
		bribes = append(bribes, SlotBribe{
			Slot:          uint64(i + 80),
			ValueWei:      big.NewInt(int64(i)),
			BuilderPubkey: "0xC",
		})
	}

	// Total: 100 blocks

	// Top 1: 50/100 = 0.5
	alpha1, _, err := ComputeBuilderConcentration(bribes, 1)
	if err != nil {
		t.Fatalf("ComputeBuilderConcentration failed: %v", err)
	}
	if alpha1 != 0.5 {
		t.Errorf("expected alpha=0.5 for top-1, got %f", alpha1)
	}

	// Top 2: 80/100 = 0.8
	alpha2, _, err := ComputeBuilderConcentration(bribes, 2)
	if err != nil {
		t.Fatalf("ComputeBuilderConcentration failed: %v", err)
	}
	if alpha2 != 0.8 {
		t.Errorf("expected alpha=0.8 for top-2, got %f", alpha2)
	}

	// Top 3: 100/100 = 1.0
	alpha3, _, err := ComputeBuilderConcentration(bribes, 3)
	if err != nil {
		t.Fatalf("ComputeBuilderConcentration failed: %v", err)
	}
	if alpha3 != 1.0 {
		t.Errorf("expected alpha=1.0 for top-3, got %f", alpha3)
	}
}

// TestGetTopBuilders verifies convenience function.
func TestGetTopBuilders(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(200), BuilderPubkey: "0xA"},
		{Slot: 3, ValueWei: big.NewInt(300), BuilderPubkey: "0xA"},
		{Slot: 4, ValueWei: big.NewInt(400), BuilderPubkey: "0xB"},
		{Slot: 5, ValueWei: big.NewInt(500), BuilderPubkey: "0xC"},
	}

	top2, err := GetTopBuilders(bribes, 2)
	if err != nil {
		t.Fatalf("GetTopBuilders failed: %v", err)
	}

	if len(top2) != 2 {
		t.Errorf("expected 2 builders, got %d", len(top2))
	}

	if top2[0].BuilderPubkey != "0xA" {
		t.Errorf("expected top builder 0xA, got %s", top2[0].BuilderPubkey)
	}
}

// TestGetBuilderDiversity verifies diversity metric.
func TestGetBuilderDiversity(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(200), BuilderPubkey: "0xA"},
		{Slot: 3, ValueWei: big.NewInt(300), BuilderPubkey: "0xB"},
		{Slot: 4, ValueWei: big.NewInt(400), BuilderPubkey: "0xC"},
		{Slot: 5, ValueWei: big.NewInt(500), BuilderPubkey: "0xB"},
	}

	diversity := GetBuilderDiversity(bribes)
	if diversity != 3 {
		t.Errorf("expected diversity=3, got %d", diversity)
	}
}

// TestGetBuilderDiversity_EmptyPubkeys verifies handling of empty pubkeys.
func TestGetBuilderDiversity_EmptyPubkeys(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: ""},
		{Slot: 2, ValueWei: big.NewInt(200), BuilderPubkey: ""},
		{Slot: 3, ValueWei: big.NewInt(300), BuilderPubkey: "0xA"},
	}

	diversity := GetBuilderDiversity(bribes)
	// Should count "unknown" and "0xA"
	if diversity != 2 {
		t.Errorf("expected diversity=2, got %d", diversity)
	}
}
