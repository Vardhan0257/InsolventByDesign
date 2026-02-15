package analysis

import (
	"fmt"
	"math"
	"math/rand"
)

// MonteCarloResult contains simulation results.
type MonteCarloResult struct {
	ExpectedProfit        float64
	ProfitStdDev          float64
	ProbabilityProfitable float64
	ValueAtRisk95         float64
	MedianProfit          float64
	MaxProfit             float64
	MaxLoss               float64
}

// SimulateAttackOutcomes runs Monte Carlo simulation of attack profitability.
func SimulateAttackOutcomes(
	censorshipCostETH float64,
	bridgeTVLUSD float64,
	ethPriceUSD float64,
	successProbability float64,
	numSimulations int,
) MonteCarloResult {

	censorshipCostUSD := censorshipCostETH * ethPriceUSD

	profits := make([]float64, numSimulations)
	profitableCount := 0

	for i := 0; i < numSimulations; i++ {
		// Simulate success (1) or failure (0)
		success := 0.0
		if rand.Float64() < successProbability {
			success = 1.0
			profitableCount++
		}

		// Profit = success * TVL - cost
		profit := success*bridgeTVLUSD - censorshipCostUSD
		profits[i] = profit
	}

	// Compute statistics
	mean := mean(profits)
	stdDev := stdDev(profits, mean)

	// Sort for percentiles
	sortedProfits := make([]float64, len(profits))
	copy(sortedProfits, profits)
	sortFloat64Slice(sortedProfits)

	return MonteCarloResult{
		ExpectedProfit:        mean,
		ProfitStdDev:          stdDev,
		ProbabilityProfitable: float64(profitableCount) / float64(numSimulations),
		ValueAtRisk95:         percentile(sortedProfits, 5),
		MedianProfit:          percentile(sortedProfits, 50),
		MaxProfit:             sortedProfits[len(sortedProfits)-1],
		MaxLoss:               sortedProfits[0],
	}
}

// OptimalAttackDuration finds the duration that maximizes expected profit.
type OptimalAttackResult struct {
	OptimalDurationSlots int
	ExpectedProfit       float64
	CensorshipCostETH    float64
	SuccessProbability   float64
}

// FindOptimalAttackDuration calculates optimal attack duration.
func FindOptimalAttackDuration(
	avgBribeETH float64,
	bridgeTVLUSD float64,
	ethPriceUSD float64,
	successProbBase float64,
	maxDurationSlots int,
	decayConstant float64,
) OptimalAttackResult {

	bestDuration := 0
	bestProfit := math.Inf(-1)
	bestCost := 0.0
	bestSuccessProb := 0.0

	// Check every 300 slots (~1 hour)
	for tau := 300; tau <= maxDurationSlots; tau += 300 {
		// Cost = average bribe * duration
		cost := avgBribeETH * float64(tau)
		costUSD := cost * ethPriceUSD

		// Success probability decays with duration
		// p(tau) = p_base * exp(-tau / decayConstant)
		successProb := successProbBase * math.Exp(-float64(tau)/decayConstant)

		// Expected profit = success_prob * TVL - cost
		expectedProfit := successProb*bridgeTVLUSD - costUSD

		if expectedProfit > bestProfit {
			bestProfit = expectedProfit
			bestDuration = tau
			bestCost = cost
			bestSuccessProb = successProb
		}
	}

	return OptimalAttackResult{
		OptimalDurationSlots: bestDuration,
		ExpectedProfit:       bestProfit,
		CensorshipCostETH:    bestCost,
		SuccessProbability:   bestSuccessProb,
	}
}

// ProfitabilityMatrix generates a 2D profitability landscape.
type ProfitabilityPoint struct {
	TVLUSD             float64
	SuccessProbability float64
	ExpectedProfitUSD  float64
}

// ComputeProfitabilityMatrix generates profit estimates across parameters.
func ComputeProfitabilityMatrix(
	censorshipCostETH float64,
	ethPriceUSD float64,
	tvlMin, tvlMax float64,
	tvlSteps int,
	probMin, probMax float64,
	probSteps int,
) []ProfitabilityPoint {

	censorshipCostUSD := censorshipCostETH * ethPriceUSD
	points := make([]ProfitabilityPoint, 0, tvlSteps*probSteps)

	tvlStep := (tvlMax - tvlMin) / float64(tvlSteps-1)
	probStep := (probMax - probMin) / float64(probSteps-1)

	for i := 0; i < tvlSteps; i++ {
		tvl := tvlMin + float64(i)*tvlStep

		for j := 0; j < probSteps; j++ {
			prob := probMin + float64(j)*probStep

			expectedProfit := prob*tvl - censorshipCostUSD

			points = append(points, ProfitabilityPoint{
				TVLUSD:             tvl,
				SuccessProbability: prob,
				ExpectedProfitUSD:  expectedProfit,
			})
		}
	}

	return points
}

// BreakevenAnalysis analyzes breakeven conditions.
type BreakevenAnalysis struct {
	BreakevenTVL        float64
	CensorshipCostETH   float64
	CensorshipCostUSD   float64
	SuccessProbability  float64
	ProfitMarginPercent float64
}

// ComputeBreakevenAnalysis calculates breakeven TVL and margins.
func ComputeBreakevenAnalysis(
	censorshipCostETH float64,
	ethPriceUSD float64,
	successProbability float64,
	currentBridgeTVL float64,
) BreakevenAnalysis {

	censorshipCostUSD := censorshipCostETH * ethPriceUSD
	breakevenTVL := censorshipCostUSD / successProbability

	// Profit margin = (TVL - breakeven) / TVL * 100
	profitMargin := 0.0
	if currentBridgeTVL > 0 {
		profitMargin = (currentBridgeTVL - breakevenTVL) / currentBridgeTVL * 100
	}

	return BreakevenAnalysis{
		BreakevenTVL:        breakevenTVL,
		CensorshipCostETH:   censorshipCostETH,
		CensorshipCostUSD:   censorshipCostUSD,
		SuccessProbability:  successProbability,
		ProfitMarginPercent: profitMargin,
	}
}

// PrintMonteCarloResult prints formatted simulation results.
func PrintMonteCarloResult(result MonteCarloResult) {
	fmt.Println("Monte Carlo Simulation Results")
	fmt.Println("================================")
	fmt.Printf("Expected Profit:    $%.2f\n", result.ExpectedProfit)
	fmt.Printf("Profit Std Dev:     $%.2f\n", result.ProfitStdDev)
	fmt.Printf("Probability Profit: %.2f%%\n", result.ProbabilityProfitable*100)
	fmt.Printf("95%% VaR:            $%.2f\n", result.ValueAtRisk95)
	fmt.Printf("Median Profit:      $%.2f\n", result.MedianProfit)
	fmt.Printf("Max Profit:         $%.2f\n", result.MaxProfit)
	fmt.Printf("Max Loss:           $%.2f\n", result.MaxLoss)
}

// Helper functions

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stdDev(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(values)))
}

func sortFloat64Slice(data []float64) {
	// Simple bubble sort (fine for our use case)
	n := len(data)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if data[j] > data[j+1] {
				data[j], data[j+1] = data[j+1], data[j]
			}
		}
	}
}
