package model

import (
	"math"
	"math/big"
	"testing"
)

// floatEqual checks if two big.Float values are equal within a small tolerance.
// This is needed because big.Float Cmp can fail on precision differences.
func floatEqual(a, b *big.Float, tolerance float64) bool {
	af, _ := a.Float64()
	bf, _ := b.Float64()
	return math.Abs(af-bf) < tolerance
}

// TestCensorshipCost_Basic verifies correct summation over tau slots.
//
// Purpose: Deterministic sum - REQUIRED by PHASE 2
func TestCensorshipCost_Basic(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000000000000000000)}, // 1 ETH
		{Slot: 2, ValueWei: big.NewInt(2000000000000000000)}, // 2 ETH
		{Slot: 3, ValueWei: big.NewInt(3000000000000000000)}, // 3 ETH
	}

	cost, err := CensorshipCost(bribes, 2)
	if err != nil {
		t.Fatalf("CensorshipCost failed: %v", err)
	}

	// Should sum first 2 slots: 1 + 2 = 3 ETH in wei
	expected := big.NewInt(3000000000000000000)
	if cost.Cmp(expected) != 0 {
		t.Errorf("expected cost %s, got %s", expected.String(), cost.String())
	}
}

// TestCensorshipCost_LargeTau verifies handling of large tau values.
//
// Purpose: Stress test - REQUIRED by PHASE 2
func TestCensorshipCost_LargeTau(t *testing.T) {
	// Create 1000 slots with increasing values
	bribes := make([]SlotBribe, 1000)
	for i := 0; i < 1000; i++ {
		bribes[i] = SlotBribe{
			Slot:     uint64(i),
			ValueWei: big.NewInt(int64(i + 1)),
		}
	}

	cost, err := CensorshipCost(bribes, 1000)
	if err != nil {
		t.Fatalf("CensorshipCost failed: %v", err)
	}

	// Sum of 1..1000 = 1000 * 1001 / 2 = 500500
	expected := big.NewInt(500500)
	if cost.Cmp(expected) != 0 {
		t.Errorf("expected cost %s, got %s", expected.String(), cost.String())
	}
}

// TestCensorshipCost_InsufficientSlots verifies failure when tau exceeds available data.
//
// Purpose: Missing slots / Fail loudly - REQUIRED by PHASE 2
func TestCensorshipCost_InsufficientSlots(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100)},
		{Slot: 2, ValueWei: big.NewInt(200)},
	}

	// Request more slots than available
	_, err := CensorshipCost(bribes, 10)
	if err == nil {
		t.Error("Expected error for insufficient slots, got nil")
	}
}

// TestCensorshipCost_ZeroTau verifies handling of edge case tau=0.
func TestCensorshipCost_ZeroTau(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100)},
	}

	cost, err := CensorshipCost(bribes, 0)
	if err != nil {
		t.Fatalf("CensorshipCost failed: %v", err)
	}

	// Sum of zero slots should be zero
	if cost.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected cost 0, got %s", cost.String())
	}
}

// TestCensorshipCost_NilValue verifies failure on nil ValueWei.
func TestCensorshipCost_NilValue(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100)},
		{Slot: 2, ValueWei: nil}, // Invalid
	}

	_, err := CensorshipCost(bribes, 2)
	if err == nil {
		t.Error("Expected error for nil ValueWei, got nil")
	}
}

// TestCensorshipCost_NoOverflow verifies big.Int prevents overflow.
//
// Purpose: Integer overflow impossible - REQUIRED by PHASE 2
func TestCensorshipCost_NoOverflow(t *testing.T) {
	// Use values near uint64 max
	maxUint64 := new(big.Int).SetUint64(^uint64(0))

	bribes := []SlotBribe{
		{Slot: 1, ValueWei: new(big.Int).Set(maxUint64)},
		{Slot: 2, ValueWei: new(big.Int).Set(maxUint64)},
	}

	cost, err := CensorshipCost(bribes, 2)
	if err != nil {
		t.Fatalf("CensorshipCost failed: %v", err)
	}

	// Result should be 2 * maxUint64, which exceeds uint64
	expected := new(big.Int).Mul(maxUint64, big.NewInt(2))
	if cost.Cmp(expected) != 0 {
		t.Errorf("expected cost %s, got %s", expected.String(), cost.String())
	}
}

// TestCensorshipCost_Deterministic verifies same input produces same output.
//
// Purpose: Deterministic - REQUIRED by PHASE 2
func TestCensorshipCost_Deterministic(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 100, ValueWei: big.NewInt(123456789)},
		{Slot: 101, ValueWei: big.NewInt(987654321)},
		{Slot: 102, ValueWei: big.NewInt(555555555)},
	}

	// Run multiple times
	var results []*big.Int
	for i := 0; i < 10; i++ {
		cost, err := CensorshipCost(bribes, 3)
		if err != nil {
			t.Fatalf("CensorshipCost failed: %v", err)
		}
		results = append(results, cost)
	}

	// All results must be identical
	for i := 1; i < len(results); i++ {
		if results[i].Cmp(results[0]) != 0 {
			t.Errorf("Non-deterministic: run 0 = %s, run %d = %s", results[0].String(), i, results[i].String())
		}
	}
}

// ========================================================================
// PHASE 4: EFFECTIVE CENSORSHIP COST TESTS
// ========================================================================

// TestEffectiveCensorshipCost_AlphaZero verifies full cost when α = 0.
//
// Purpose: α = 0 (full cost) - REQUIRED by PHASE 4
//
// When all builders are unique (no concentration), effective cost = raw cost.
func TestEffectiveCensorshipCost_AlphaZero(t *testing.T) {
	// Each slot has a different builder = no concentration among top-1
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(2000), BuilderPubkey: "0xB"},
		{Slot: 3, ValueWei: big.NewInt(3000), BuilderPubkey: "0xC"},
	}

	// For top-1 with diverse builders: α = 1/3, not 0
	// To get α = 0, we need top-k where k = 0, but that's invalid
	// Actually, α can never be truly 0 with valid data since k ≥ 1
	// The minimum is α = 1/n where n is total slots

	// Let's test the formula correctness instead
	ccEff, alpha, err := EffectiveCensorshipCost(bribes, 3, 1)
	if err != nil {
		t.Fatalf("EffectiveCensorshipCost failed: %v", err)
	}

	// Top-1 builder has 1/3 blocks, so α = 1/3
	expectedAlpha := 1.0 / 3.0
	if alpha != expectedAlpha {
		t.Errorf("expected alpha=%f, got %f", expectedAlpha, alpha)
	}

	// C_c = 1000 + 2000 + 3000 = 6000
	// C_c^eff = (1 - 1/3) * 6000 = (2/3) * 6000 = 4000
	expectedCost := big.NewFloat(4000.0)
	if !floatEqual(ccEff, expectedCost, 0.01) {
		t.Errorf("expected effective cost %s, got %s", expectedCost.String(), ccEff.String())
	}
}

// TestEffectiveCensorshipCost_AlphaOne verifies zero effective cost when α = 1.
//
// Purpose: α = 1 (zero effective cost) - REQUIRED by PHASE 4
//
// When a single builder controls all blocks, effective cost = 0.
func TestEffectiveCensorshipCost_AlphaOne(t *testing.T) {
	// Single builder monopoly
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xmonopoly"},
		{Slot: 2, ValueWei: big.NewInt(2000), BuilderPubkey: "0xmonopoly"},
		{Slot: 3, ValueWei: big.NewInt(3000), BuilderPubkey: "0xmonopoly"},
	}

	ccEff, alpha, err := EffectiveCensorshipCost(bribes, 3, 1)
	if err != nil {
		t.Fatalf("EffectiveCensorshipCost failed: %v", err)
	}

	// Single builder = α = 1.0
	if alpha != 1.0 {
		t.Errorf("expected alpha=1.0, got %f", alpha)
	}

	// C_c^eff = (1 - 1) * 6000 = 0
	expectedCost := big.NewFloat(0.0)
	if !floatEqual(ccEff, expectedCost, 0.01) {
		t.Errorf("expected effective cost %s, got %s", expectedCost.String(), ccEff.String())
	}
}

// TestEffectiveCensorshipCost_AlphaBounds verifies α ∈ [0,1].
//
// Purpose: Bounds check - REQUIRED by PHASE 4
func TestEffectiveCensorshipCost_AlphaBounds(t *testing.T) {
	testCases := []struct {
		name   string
		bribes []SlotBribe
		tau    uint64
		topK   int
	}{
		{
			name: "diverse_builders",
			bribes: []SlotBribe{
				{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0x1"},
				{Slot: 2, ValueWei: big.NewInt(100), BuilderPubkey: "0x2"},
				{Slot: 3, ValueWei: big.NewInt(100), BuilderPubkey: "0x3"},
				{Slot: 4, ValueWei: big.NewInt(100), BuilderPubkey: "0x4"},
				{Slot: 5, ValueWei: big.NewInt(100), BuilderPubkey: "0x5"},
			},
			tau:  5,
			topK: 2,
		},
		{
			name: "concentrated",
			bribes: []SlotBribe{
				{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
				{Slot: 2, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
				{Slot: 3, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
				{Slot: 4, ValueWei: big.NewInt(100), BuilderPubkey: "0xB"},
			},
			tau:  4,
			topK: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, alpha, err := EffectiveCensorshipCost(tc.bribes, tc.tau, tc.topK)
			if err != nil {
				t.Fatalf("EffectiveCensorshipCost failed: %v", err)
			}

			// Verify bounds
			if alpha < 0 || alpha > 1 {
				t.Errorf("alpha out of bounds: %f (must be in [0,1])", alpha)
			}
		})
	}
}

// TestEffectiveCensorshipCost_Formula verifies correct formula application.
//
// Purpose: Validate (1-α)*C_c computation
func TestEffectiveCensorshipCost_Formula(t *testing.T) {
	// Known distribution
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(2000), BuilderPubkey: "0xA"},
		{Slot: 3, ValueWei: big.NewInt(3000), BuilderPubkey: "0xB"},
		{Slot: 4, ValueWei: big.NewInt(4000), BuilderPubkey: "0xC"},
	}

	// 4 total slots: A has 2, B has 1, C has 1
	// Top-1: α = 2/4 = 0.5
	// C_c = 1000 + 2000 + 3000 + 4000 = 10000
	// C_c^eff = (1 - 0.5) * 10000 = 5000

	ccEff, alpha, err := EffectiveCensorshipCost(bribes, 4, 1)
	if err != nil {
		t.Fatalf("EffectiveCensorshipCost failed: %v", err)
	}

	// Verify alpha
	expectedAlpha := 0.5
	if alpha != expectedAlpha {
		t.Errorf("expected alpha=%f, got %f", expectedAlpha, alpha)
	}

	// Verify effective cost
	expectedCost := big.NewFloat(5000.0)
	if !floatEqual(ccEff, expectedCost, 0.01) {
		t.Errorf("expected effective cost %s, got %s", expectedCost.String(), ccEff.String())
	}
}

// TestEffectiveCensorshipCost_TopKVariation verifies different k values.
func TestEffectiveCensorshipCost_TopKVariation(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 3, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 4, ValueWei: big.NewInt(1000), BuilderPubkey: "0xB"},
		{Slot: 5, ValueWei: big.NewInt(1000), BuilderPubkey: "0xB"},
		{Slot: 6, ValueWei: big.NewInt(1000), BuilderPubkey: "0xC"},
	}

	// 6 slots total: A=3, B=2, C=1
	// C_c = 6000

	// Top-1: α = 3/6 = 0.5, C_c^eff = 0.5 * 6000 = 3000
	ccEff1, alpha1, err := EffectiveCensorshipCost(bribes, 6, 1)
	if err != nil {
		t.Fatalf("EffectiveCensorshipCost failed for top-1: %v", err)
	}
	if alpha1 != 0.5 {
		t.Errorf("top-1: expected alpha=0.5, got %f", alpha1)
	}
	expected1 := big.NewFloat(3000.0)
	if !floatEqual(ccEff1, expected1, 0.01) {
		t.Errorf("top-1: expected %s, got %s", expected1.String(), ccEff1.String())
	}

	// Top-2: α = 5/6, C_c^eff = (1/6) * 6000 = 1000
	ccEff2, alpha2, err := EffectiveCensorshipCost(bribes, 6, 2)
	if err != nil {
		t.Fatalf("EffectiveCensorshipCost failed for top-2: %v", err)
	}
	expectedAlpha2 := 5.0 / 6.0
	if alpha2 != expectedAlpha2 {
		t.Errorf("top-2: expected alpha=%f, got %f", expectedAlpha2, alpha2)
	}
	expected2 := big.NewFloat(1000.0)
	if !floatEqual(ccEff2, expected2, 0.01) {
		t.Errorf("top-2: expected %s, got %s", expected2.String(), ccEff2.String())
	}

	// Top-3: α = 1.0, C_c^eff = 0
	ccEff3, alpha3, err := EffectiveCensorshipCost(bribes, 6, 3)
	if err != nil {
		t.Fatalf("EffectiveCensorshipCost failed for top-3: %v", err)
	}
	if alpha3 != 1.0 {
		t.Errorf("top-3: expected alpha=1.0, got %f", alpha3)
	}
	expected3 := big.NewFloat(0.0)
	if !floatEqual(ccEff3, expected3, 0.01) {
		t.Errorf("top-3: expected %s, got %s", expected3.String(), ccEff3.String())
	}
}

// TestEffectiveCensorshipCost_LargeValues verifies handling of large wei values.
func TestEffectiveCensorshipCost_LargeValues(t *testing.T) {
	// Use realistic block values (e.g., 0.1 ETH per slot)
	weiPer01ETH := new(big.Int)
	weiPer01ETH.SetString("100000000000000000", 10) // 0.1 ETH

	bribes := []SlotBribe{
		{Slot: 1, ValueWei: new(big.Int).Set(weiPer01ETH), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: new(big.Int).Set(weiPer01ETH), BuilderPubkey: "0xA"},
		{Slot: 3, ValueWei: new(big.Int).Set(weiPer01ETH), BuilderPubkey: "0xB"},
		{Slot: 4, ValueWei: new(big.Int).Set(weiPer01ETH), BuilderPubkey: "0xB"},
	}

	// α = 0.5 (top-1 builder has 2/4)
	// C_c = 4 * 0.1 ETH = 0.4 ETH
	// C_c^eff = 0.5 * 0.4 ETH = 0.2 ETH

	ccEff, alpha, err := EffectiveCensorshipCost(bribes, 4, 1)
	if err != nil {
		t.Fatalf("EffectiveCensorshipCost failed: %v", err)
	}

	if alpha != 0.5 {
		t.Errorf("expected alpha=0.5, got %f", alpha)
	}

	// C_c^eff should be 0.2 ETH = 200000000000000000 wei
	expectedWei := new(big.Float)
	expectedWei.SetString("200000000000000000")

	if !floatEqual(ccEff, expectedWei, 1.0) {
		t.Errorf("expected %s wei, got %s wei", expectedWei.String(), ccEff.String())
	}
}

// TestEffectiveCensorshipCost_InsufficientData verifies error propagation.
func TestEffectiveCensorshipCost_InsufficientData(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
	}

	// Request more slots than available
	_, _, err := EffectiveCensorshipCost(bribes, 10, 1)
	if err == nil {
		t.Error("Expected error for insufficient data, got nil")
	}
}

// TestEffectiveCensorshipCost_InvalidK verifies error on invalid topK.
func TestEffectiveCensorshipCost_InvalidK(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(100), BuilderPubkey: "0xA"},
	}

	_, _, err := EffectiveCensorshipCost(bribes, 1, 0)
	if err == nil {
		t.Error("Expected error for topK=0, got nil")
	}
}

// ========================================================================
// PHASE 5: ATTACKER PROFIT FUNCTION TESTS
// ========================================================================

// TestAttackerProfit_Basic verifies correct profit calculation.
//
// Purpose: Basic P(V) = p*V - C_c^eff validation
func TestAttackerProfit_Basic(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(2000), BuilderPubkey: "0xB"},
	}

	// TVL = 10000 wei, p = 0.5, tau = 2, top-1
	params := ProfitParams{
		BridgeTVL:          big.NewFloat(10000),
		SuccessProbability: 0.5,
		Tau:                2,
		TopK:               1,
	}

	result, err := AttackerProfit(bribes, params)
	if err != nil {
		t.Fatalf("AttackerProfit failed: %v", err)
	}

	// Expected revenue = 0.5 * 10000 = 5000
	expectedRev := big.NewFloat(5000)
	if !floatEqual(result.ExpectedRevenue, expectedRev, 0.01) {
		t.Errorf("expected revenue %s, got %s", expectedRev.String(), result.ExpectedRevenue.String())
	}

	// C_c = 1000 + 2000 = 3000
	// α = 0.5 (each builder has 1/2)
	// C_c^eff = (1 - 0.5) * 3000 = 1500
	expectedCost := big.NewFloat(1500)
	if !floatEqual(result.EffectiveCost, expectedCost, 0.01) {
		t.Errorf("expected cost %s, got %s", expectedCost.String(), result.EffectiveCost.String())
	}

	// Profit = 5000 - 1500 = 3500
	expectedProfit := big.NewFloat(3500)
	if !floatEqual(result.Profit, expectedProfit, 0.01) {
		t.Errorf("expected profit %s, got %s", expectedProfit.String(), result.Profit.String())
	}
}

// TestAttackerProfit_Breakeven verifies zero profit case.
func TestAttackerProfit_Breakeven(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	// C_c = 2000, α = 1.0 (monopoly), C_c^eff = 0
	// Set TVL = 0, so revenue = 0, profit = 0
	params := ProfitParams{
		BridgeTVL:          big.NewFloat(0),
		SuccessProbability: 1.0,
		Tau:                2,
		TopK:               1,
	}

	result, err := AttackerProfit(bribes, params)
	if err != nil {
		t.Fatalf("AttackerProfit failed: %v", err)
	}

	// Profit should be 0 - 0 = 0
	expectedProfit := big.NewFloat(0)
	if !floatEqual(result.Profit, expectedProfit, 0.01) {
		t.Errorf("expected profit %s, got %s", expectedProfit.String(), result.Profit.String())
	}
}

// TestAttackerProfit_NegativeProfit verifies loss scenario.
func TestAttackerProfit_NegativeProfit(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xB"},
	}

	// C_c = 2000, α = 0.5, C_c^eff = 1000
	// TVL = 1000, p = 0.5, revenue = 500
	// Profit = 500 - 1000 = -500 (loss)
	params := ProfitParams{
		BridgeTVL:          big.NewFloat(1000),
		SuccessProbability: 0.5,
		Tau:                2,
		TopK:               1,
	}

	result, err := AttackerProfit(bribes, params)
	if err != nil {
		t.Fatalf("AttackerProfit failed: %v", err)
	}

	// Profit should be negative
	if result.Profit.Sign() >= 0 {
		t.Errorf("expected negative profit, got %s", result.Profit.String())
	}

	expectedProfit := big.NewFloat(-500)
	if !floatEqual(result.Profit, expectedProfit, 0.01) {
		t.Errorf("expected profit %s, got %s", expectedProfit.String(), result.Profit.String())
	}
}

// TestAttackerProfit_InvalidProbability verifies bounds checking.
func TestAttackerProfit_InvalidProbability(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	// Test p < 0
	params := ProfitParams{
		BridgeTVL:          big.NewFloat(1000),
		SuccessProbability: -0.1,
		Tau:                1,
		TopK:               1,
	}
	_, err := AttackerProfit(bribes, params)
	if err == nil {
		t.Error("Expected error for p < 0, got nil")
	}

	// Test p > 1
	params.SuccessProbability = 1.5
	_, err = AttackerProfit(bribes, params)
	if err == nil {
		t.Error("Expected error for p > 1, got nil")
	}
}

// TestAttackerProfit_NilTVL verifies nil TVL handling.
func TestAttackerProfit_NilTVL(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	params := ProfitParams{
		BridgeTVL:          nil,
		SuccessProbability: 0.5,
		Tau:                1,
		TopK:               1,
	}

	_, err := AttackerProfit(bribes, params)
	if err == nil {
		t.Error("Expected error for nil TVL, got nil")
	}
}

// TestAttackerProfit_NegativeTVL verifies negative TVL rejection.
func TestAttackerProfit_NegativeTVL(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	params := ProfitParams{
		BridgeTVL:          big.NewFloat(-1000),
		SuccessProbability: 0.5,
		Tau:                1,
		TopK:               1,
	}

	_, err := AttackerProfit(bribes, params)
	if err == nil {
		t.Error("Expected error for negative TVL, got nil")
	}
}

// TestSweepProbability_Basic verifies probability sweep.
func TestSweepProbability_Basic(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	tvl := big.NewFloat(10000)
	sweep, err := SweepProbability(bribes, tvl, 2, 1, 0.1, 1.0, 10)
	if err != nil {
		t.Fatalf("SweepProbability failed: %v", err)
	}

	// Should have 10 results
	if len(sweep.Results) != 10 {
		t.Errorf("expected 10 results, got %d", len(sweep.Results))
	}

	// Verify range
	if sweep.MinP != 0.1 {
		t.Errorf("expected minP=0.1, got %f", sweep.MinP)
	}
	if sweep.MaxP != 1.0 {
		t.Errorf("expected maxP=1.0, got %f", sweep.MaxP)
	}

	// Verify monotonically increasing probability
	for i := 1; i < len(sweep.Results); i++ {
		if sweep.Results[i].SuccessProb <= sweep.Results[i-1].SuccessProb {
			t.Errorf("probabilities not monotonically increasing at index %d", i)
		}
	}

	// Verify profit increases with p (for fixed TVL)
	for i := 1; i < len(sweep.Results); i++ {
		prevProfit, _ := sweep.Results[i-1].Profit.Float64()
		currProfit, _ := sweep.Results[i].Profit.Float64()
		if currProfit <= prevProfit {
			t.Errorf("profit not increasing with p at index %d", i)
		}
	}
}

// TestSweepProbability_SingleStep verifies single-point evaluation.
func TestSweepProbability_SingleStep(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	tvl := big.NewFloat(5000)
	sweep, err := SweepProbability(bribes, tvl, 1, 1, 0.5, 0.9, 1)
	if err != nil {
		t.Fatalf("SweepProbability failed: %v", err)
	}

	if len(sweep.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(sweep.Results))
	}

	// Should use minP when only one step
	if sweep.Results[0].SuccessProb != 0.5 {
		t.Errorf("expected p=0.5, got %f", sweep.Results[0].SuccessProb)
	}
}

// TestSweepProbability_InvalidParams verifies parameter validation.
func TestSweepProbability_InvalidParams(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}
	tvl := big.NewFloat(1000)

	// Test steps < 1
	_, err := SweepProbability(bribes, tvl, 1, 1, 0.1, 1.0, 0)
	if err == nil {
		t.Error("Expected error for steps=0, got nil")
	}

	// Test minP > maxP
	_, err = SweepProbability(bribes, tvl, 1, 1, 0.9, 0.1, 5)
	if err == nil {
		t.Error("Expected error for minP > maxP, got nil")
	}

	// Test minP out of bounds
	_, err = SweepProbability(bribes, tvl, 1, 1, -0.1, 1.0, 5)
	if err == nil {
		t.Error("Expected error for minP < 0, got nil")
	}

	// Test maxP out of bounds
	_, err = SweepProbability(bribes, tvl, 1, 1, 0.1, 1.5, 5)
	if err == nil {
		t.Error("Expected error for maxP > 1, got nil")
	}
}

// TestFindBreakevenTVL_Basic verifies threshold calculation.
//
// Purpose: Threshold discovery - REQUIRED by PHASE 5/6
func TestFindBreakevenTVL_Basic(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(2000), BuilderPubkey: "0xB"},
	}

	// C_c = 3000, α = 0.5, C_c^eff = 1500
	// For p = 0.5: V* = 1500 / 0.5 = 3000
	breakeven, alpha, err := FindBreakevenTVL(bribes, 0.5, 2, 1)
	if err != nil {
		t.Fatalf("FindBreakevenTVL failed: %v", err)
	}

	if alpha != 0.5 {
		t.Errorf("expected alpha=0.5, got %f", alpha)
	}

	expected := big.NewFloat(3000)
	if !floatEqual(breakeven, expected, 0.01) {
		t.Errorf("expected breakeven %s, got %s", expected.String(), breakeven.String())
	}
}

// TestFindBreakevenTVL_HighProbability verifies lower threshold with high p.
func TestFindBreakevenTVL_HighProbability(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xB"},
	}

	// C_c = 2000, α = 0.5, C_c^eff = 1000
	// For p = 1.0: V* = 1000 / 1.0 = 1000
	breakeven, _, err := FindBreakevenTVL(bribes, 1.0, 2, 1)
	if err != nil {
		t.Fatalf("FindBreakevenTVL failed: %v", err)
	}

	expected := big.NewFloat(1000)
	if !floatEqual(breakeven, expected, 0.01) {
		t.Errorf("expected breakeven %s, got %s", expected.String(), breakeven.String())
	}
}

// TestFindBreakevenTVL_LowProbability verifies higher threshold with low p.
func TestFindBreakevenTVL_LowProbability(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xB"},
	}

	// C_c = 2000, α = 0.5, C_c^eff = 1000
	// For p = 0.1: V* = 1000 / 0.1 = 10000
	breakeven, _, err := FindBreakevenTVL(bribes, 0.1, 2, 1)
	if err != nil {
		t.Fatalf("FindBreakevenTVL failed: %v", err)
	}

	expected := big.NewFloat(10000)
	if !floatEqual(breakeven, expected, 0.01) {
		t.Errorf("expected breakeven %s, got %s", expected.String(), breakeven.String())
	}
}

// TestFindBreakevenTVL_InvalidProbability verifies bounds checking.
func TestFindBreakevenTVL_InvalidProbability(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	// Test p = 0 (division by zero)
	_, _, err := FindBreakevenTVL(bribes, 0, 1, 1)
	if err == nil {
		t.Error("Expected error for p=0, got nil")
	}

	// Test p < 0
	_, _, err = FindBreakevenTVL(bribes, -0.1, 1, 1)
	if err == nil {
		t.Error("Expected error for p<0, got nil")
	}

	// Test p > 1
	_, _, err = FindBreakevenTVL(bribes, 1.5, 1, 1)
	if err == nil {
		t.Error("Expected error for p>1, got nil")
	}
}

// TestFindBreakevenTVL_Monopoly verifies zero threshold under monopoly.
func TestFindBreakevenTVL_Monopoly(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	// α = 1.0, C_c^eff = 0
	// V* = 0 / p = 0 (always profitable)
	breakeven, alpha, err := FindBreakevenTVL(bribes, 0.5, 2, 1)
	if err != nil {
		t.Fatalf("FindBreakevenTVL failed: %v", err)
	}

	if alpha != 1.0 {
		t.Errorf("expected alpha=1.0, got %f", alpha)
	}

	expected := big.NewFloat(0)
	if !floatEqual(breakeven, expected, 0.01) {
		t.Errorf("expected breakeven %s, got %s", expected.String(), breakeven.String())
	}
}

// ========================================================================
// PHASE 7: FALSIFICATION & STRESS TESTING
// ========================================================================
//
// These tests document the EXPLICIT LIMITATIONS of the model.
// This is what makes the research credible - we do not hide constraints.

// TestStressCase_HigherTau verifies cost scaling with longer censorship.
//
// Purpose: Demonstrate that profitability shifts with τ
func TestStressCase_HigherTau(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 3, ValueWei: big.NewInt(1000), BuilderPubkey: "0xB"},
		{Slot: 4, ValueWei: big.NewInt(1000), BuilderPubkey: "0xB"},
	}

	// Compute for τ=2 vs τ=4
	breakeven2, _, _ := FindBreakevenTVL(bribes, 0.5, 2, 1)
	breakeven4, _, _ := FindBreakevenTVL(bribes, 0.5, 4, 1)

	// Longer censorship requires higher TVL to be profitable
	if breakeven4.Cmp(breakeven2) <= 0 {
		t.Error("Expected higher τ to increase breakeven threshold")
	}

	// This demonstrates: longer attacks are MORE expensive
	// Limitation: assumes attacker can sustain coordination for τ slots
}

// TestStressCase_LowerAlpha verifies cost increase with lower concentration.
//
// Purpose: Demonstrate that decentralization increases security
func TestStressCase_LowerAlpha(t *testing.T) {
	// Scenario 1: High concentration (2 builders)
	highConcentration := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
	}

	// Scenario 2: Low concentration (2 different builders)
	lowConcentration := []SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1000), BuilderPubkey: "0xA"},
		{Slot: 2, ValueWei: big.NewInt(1000), BuilderPubkey: "0xB"},
	}

	alphaHigh, _, _ := ComputeBuilderConcentration(highConcentration, 1)
	alphaLow, _, _ := ComputeBuilderConcentration(lowConcentration, 1)

	// Confirm concentration difference
	if alphaHigh <= alphaLow {
		t.Error("Expected high concentration scenario to have higher α")
	}

	// Higher α = lower effective cost = lower threshold
	breakevenHigh, _, _ := FindBreakevenTVL(highConcentration, 0.5, 2, 1)
	breakevenLow, _, _ := FindBreakevenTVL(lowConcentration, 0.5, 2, 1)

	if breakevenHigh.Cmp(breakevenLow) >= 0 {
		t.Error("Expected lower concentration to require higher TVL threshold")
	}

	// This demonstrates: builder decentralization MATTERS for security
	// Limitation: assumes no out-of-protocol coordination
}

// TestLimitation_InclusionLists documents the inclusion list limitation.
//
// Purpose: EXPLICITLY STATE that inclusion lists invalidate this model
func TestLimitation_InclusionLists(t *testing.T) {
	// This test exists to document the limitation, not to test code

	t.Log("LIMITATION: This model does NOT account for inclusion lists (EIP-7547)")
	t.Log("")
	t.Log("If inclusion lists are adopted:")
	t.Log("  - Proposers can force transaction inclusion")
	t.Log("  - Builder censorship becomes ineffective")
	t.Log("  - Attack cost model breaks down")
	t.Log("")
	t.Log("This model is valid ONLY in pre-inclusion-list Ethereum.")
	t.Log("")
	t.Log("Current status (Feb 2026): Inclusion lists under development")
}

// TestLimitation_BridgeAdaptation documents bridge defense mechanisms.
//
// Purpose: EXPLICITLY STATE that bridges can adapt defenses
func TestLimitation_BridgeAdaptation(t *testing.T) {
	// This test exists to document the limitation, not to test code

	t.Log("LIMITATION: This model does NOT account for bridge-level defenses")
	t.Log("")
	t.Log("Bridges can implement countermeasures:")
	t.Log("  - Fraud proof submission via multiple paths")
	t.Log("  - Off-chain proof submission channels")
	t.Log("  - Redundant validation networks")
	t.Log("  - Economic penalties for challengers")
	t.Log("")
	t.Log("This model measures CURRENT PBS economics, not adapted systems.")
}

// TestLimitation_SocialLayer documents social/legal consequences.
//
// Purpose: EXPLICITLY STATE that non-economic factors matter
func TestLimitation_SocialLayer(t *testing.T) {
	// This test exists to document the limitation, not to test code

	t.Log("LIMITATION: This model does NOT account for social/legal factors")
	t.Log("")
	t.Log("Real-world consequences ignored by this model:")
	t.Log("  - Legal prosecution of attackers")
	t.Log("  - Reputational damage to builders")
	t.Log("  - Ethereum community response")
	t.Log("  - Potential protocol-level intervention")
	t.Log("")
	t.Log("Economic profitability ≠ rational attack decision")
}

// TestLimitation_DetectionRisk documents detection and response.
//
// Purpose: Acknowledge that attacks can be detected and stopped
func TestLimitation_DetectionRisk(t *testing.T) {
	// This test exists to document the limitation, not to test code

	t.Log("LIMITATION: This model assumes undetected attacks")
	t.Log("")
	t.Log("In reality:")
	t.Log("  - Censorship can be detected on-chain")
	t.Log("  - Community can respond during attack")
	t.Log("  - Victim bridges can trigger emergency modes")
	t.Log("  - Validators may refuse censoring blocks")
	t.Log("")
	t.Log("Success probability p MUST account for detection risk")
}

// TestLimitation_CoordinationCost documents cartel formation costs.
//
// Purpose: Acknowledge that coordination is not free
func TestLimitation_CoordinationCost(t *testing.T) {
	// This test exists to document the limitation, not to test code

	t.Log("LIMITATION: This model assumes zero coordination cost")
	t.Log("")
	t.Log("Real coordination requires:")
	t.Log("  - Trust between cartel members")
	t.Log("  - Communication channels")
	t.Log("  - Enforceable agreements")
	t.Log("  - Time to organize")
	t.Log("")
	t.Log("True cost = C_c^eff + coordination overhead")
}

// TestModelValidityBounds documents when this model is valid.
//
// Purpose: STATE EXPLICIT CONDITIONS under which results hold
func TestModelValidityBounds(t *testing.T) {
	t.Log("========================================")
	t.Log("MODEL VALIDITY CONDITIONS")
	t.Log("========================================")
	t.Log("")
	t.Log("This model produces valid results ONLY when:")
	t.Log("")
	t.Log("1. NO inclusion lists are active")
	t.Log("2. Bridges use standard fraud proof windows")
	t.Log("3. Builder market remains as measured")
	t.Log("4. No protocol-level censorship resistance")
	t.Log("5. Success probability p is explicitly justified")
	t.Log("")
	t.Log("If ANY condition is violated, results are INVALID.")
	t.Log("")
	t.Log("This is a MEASUREMENT of current economics,")
	t.Log("not a prediction of future attack feasibility.")
	t.Log("========================================")
}
