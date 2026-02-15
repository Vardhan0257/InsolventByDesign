package model

import (
	"fmt"
	"math/big"
)

// SlotBribe represents the minimum cost required
// to exclude a transaction from a single slot.
//
// This type uses big.Int to preserve exact wei values
// with NO floating point precision loss.
type SlotBribe struct {
	Slot          uint64   // Consensus slot number
	ValueWei      *big.Int // Winning bid in wei (exact)
	BuilderPubkey string   // Builder identity for concentration analysis
}

// CensorshipCost computes the total cost required
// to censor a transaction for tau consecutive slots.
//
// Formula: C_c(τ) = Σ(t=1 to τ) b(t)
//
// Where b(t) is the winning bid for slot t.
//
// Guarantees:
// - Deterministic output (same input → same result)
// - No overflow (big.Int arithmetic)
// - Exact wei precision
// - Fails if bribes slice has fewer than tau elements
func CensorshipCost(bribes []SlotBribe, tau uint64) (*big.Int, error) {
	if uint64(len(bribes)) < tau {
		return nil, fmt.Errorf("insufficient data: need %d slots, have %d", tau, len(bribes))
	}

	total := new(big.Int)
	for i := uint64(0); i < tau; i++ {
		if bribes[i].ValueWei == nil {
			return nil, fmt.Errorf("nil ValueWei at index %d", i)
		}
		total.Add(total, bribes[i].ValueWei)
	}

	return total, nil
}

// EffectiveCensorshipCost computes the censorship cost adjusted for builder concentration.
//
// Formula: C_c^eff = (1 - α) · C_c
//
// Where:
// - C_c is the total censorship cost for tau slots
// - α is the builder concentration coefficient (top-k builders)
// - (1 - α) represents the fraction of builders not in the cartel
//
// Rationale:
// - Under builder concentration, coordinated censorship becomes cheaper
// - Top-k builders can coordinate out-of-band
// - Effective cost only accounts for bribing the remaining (1-α) fraction
//
// This is the "rent-a-cartel" economic model.
//
// Returns the effective cost as *big.Float for precision, since α is inherently float64.
func EffectiveCensorshipCost(bribes []SlotBribe, tau uint64, topK int) (*big.Float, float64, error) {
	// Compute raw censorship cost
	cc, err := CensorshipCost(bribes, tau)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to compute censorship cost: %w", err)
	}

	// Compute builder concentration
	alpha, _, err := ComputeBuilderConcentration(bribes, topK)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to compute concentration: %w", err)
	}

	// Validate alpha bounds (should always be true by construction, but verify)
	if alpha < 0 || alpha > 1 {
		return nil, 0, fmt.Errorf("invalid alpha value: %f (must be in [0,1])", alpha)
	}

	// Compute effective cost: C_c^eff = (1 - α) * C_c
	ccFloat := new(big.Float).SetInt(cc)
	discount := 1.0 - alpha
	discountFloat := big.NewFloat(discount)

	ccEff := new(big.Float).Mul(ccFloat, discountFloat)

	return ccEff, alpha, nil
}

// ProfitParams contains parameters for attacker profit calculation.
type ProfitParams struct {
	BridgeTVL          *big.Float // V: Total Value Locked in bridge (wei)
	SuccessProbability float64    // p: Probability of successful attack ∈ [0, 1]
	Tau                uint64     // τ: Censorship duration in slots
	TopK               int        // k: Number of top builders in cartel
}

// ProfitResult contains the output of profit calculation.
type ProfitResult struct {
	ExpectedRevenue *big.Float // p(V) * V
	EffectiveCost   *big.Float // C_c^eff
	Profit          *big.Float // P(V) = p(V)*V - C_c^eff
	Alpha           float64    // Builder concentration coefficient
	SuccessProb     float64    // p used in calculation
	TVL             *big.Float // V used in calculation
}

// AttackerProfit computes the expected profit from a censorship attack.
//
// Formula: P(V) = p(V) · V - C_c^eff
//
// Where:
// - V is the bridge TVL (Total Value Locked)
// - p(V) is the probability of successful attack (assumption/parameter)
// - C_c^eff is the effective censorship cost accounting for builder concentration
//
// Critical note:
// We do NOT claim to know p(V). This function evaluates profit under
// EXPLICIT assumptions about p. The caller must justify any p value used.
//
// Returns:
// - ProfitResult containing all economic parameters
// - error if computation fails
func AttackerProfit(bribes []SlotBribe, params ProfitParams) (*ProfitResult, error) {
	// Validate inputs
	if params.SuccessProbability < 0 || params.SuccessProbability > 1 {
		return nil, fmt.Errorf("invalid success probability: %f (must be in [0,1])", params.SuccessProbability)
	}
	if params.BridgeTVL == nil {
		return nil, fmt.Errorf("BridgeTVL cannot be nil")
	}
	if params.BridgeTVL.Sign() < 0 {
		return nil, fmt.Errorf("BridgeTVL cannot be negative")
	}

	// Compute effective censorship cost
	ccEff, alpha, err := EffectiveCensorshipCost(bribes, params.Tau, params.TopK)
	if err != nil {
		return nil, fmt.Errorf("failed to compute effective cost: %w", err)
	}

	// Compute expected revenue: p(V) * V
	pFloat := big.NewFloat(params.SuccessProbability)
	expectedRevenue := new(big.Float).Mul(pFloat, params.BridgeTVL)

	// Compute profit: P(V) = p(V)*V - C_c^eff
	profit := new(big.Float).Sub(expectedRevenue, ccEff)

	return &ProfitResult{
		ExpectedRevenue: expectedRevenue,
		EffectiveCost:   ccEff,
		Profit:          profit,
		Alpha:           alpha,
		SuccessProb:     params.SuccessProbability,
		TVL:             new(big.Float).Set(params.BridgeTVL),
	}, nil
}

// ProfitSweepResult contains results from sweeping probability values.
type ProfitSweepResult struct {
	Results []ProfitResult
	MinP    float64
	MaxP    float64
	Steps   int
}

// SweepProbability evaluates profit across a range of success probabilities.
//
// This function DOES NOT claim any p value is correct. It simply evaluates
// the profit function under different assumptions.
//
// Purpose: Threshold discovery - find V* where P(V) > 0
//
// Parameters:
// - bribes: slot-level bribe data
// - tvl: bridge TVL to evaluate
// - tau: censorship duration
// - topK: cartel size
// - minP, maxP: probability range to sweep
// - steps: number of evaluation points
//
// Returns sweep results for analysis.
func SweepProbability(bribes []SlotBribe, tvl *big.Float, tau uint64, topK int, minP, maxP float64, steps int) (*ProfitSweepResult, error) {
	if steps < 1 {
		return nil, fmt.Errorf("steps must be at least 1, got %d", steps)
	}
	if minP < 0 || minP > 1 {
		return nil, fmt.Errorf("minP must be in [0,1], got %f", minP)
	}
	if maxP < 0 || maxP > 1 {
		return nil, fmt.Errorf("maxP must be in [0,1], got %f", maxP)
	}
	if minP > maxP {
		return nil, fmt.Errorf("minP (%f) must be <= maxP (%f)", minP, maxP)
	}

	results := make([]ProfitResult, 0, steps)

	// Handle single step case
	if steps == 1 {
		params := ProfitParams{
			BridgeTVL:          tvl,
			SuccessProbability: minP,
			Tau:                tau,
			TopK:               topK,
		}
		result, err := AttackerProfit(bribes, params)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
		return &ProfitSweepResult{
			Results: results,
			MinP:    minP,
			MaxP:    maxP,
			Steps:   steps,
		}, nil
	}

	// Multiple steps: sweep from minP to maxP
	stepSize := (maxP - minP) / float64(steps-1)
	for i := 0; i < steps; i++ {
		p := minP + float64(i)*stepSize

		params := ProfitParams{
			BridgeTVL:          tvl,
			SuccessProbability: p,
			Tau:                tau,
			TopK:               topK,
		}

		result, err := AttackerProfit(bribes, params)
		if err != nil {
			return nil, fmt.Errorf("failed at p=%f: %w", p, err)
		}

		results = append(results, *result)
	}

	return &ProfitSweepResult{
		Results: results,
		MinP:    minP,
		MaxP:    maxP,
		Steps:   steps,
	}, nil
}

// FindBreakevenTVL finds the minimum TVL where profit becomes positive.
//
// This is the threshold V* where:
//
//	V* = min { V : P(V) > 0 } = C_c^eff / p
//
// Given a fixed p, returns the TVL threshold.
//
// WARNING: The meaning of V* depends entirely on the assumed p.
// Different p values yield different thresholds.
//
// This function implements the "kill shot" calculation from the blueprint.
func FindBreakevenTVL(bribes []SlotBribe, successProb float64, tau uint64, topK int) (*big.Float, float64, error) {
	if successProb <= 0 || successProb > 1 {
		return nil, 0, fmt.Errorf("success probability must be in (0,1], got %f", successProb)
	}

	// Compute effective censorship cost
	ccEff, alpha, err := EffectiveCensorshipCost(bribes, tau, topK)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to compute effective cost: %w", err)
	}

	// V* = C_c^eff / p
	pFloat := big.NewFloat(successProb)
	breakeven := new(big.Float).Quo(ccEff, pFloat)

	return breakeven, alpha, nil
}
