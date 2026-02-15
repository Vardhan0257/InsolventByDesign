package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"

	"insolventbydesign/internal/analysis"
	"insolventbydesign/internal/model"
)

func main() {
	// Command line flags
	var (
		dataFile    = flag.String("data", "data/bribes.json", "Input data file")
		mode        = flag.String("mode", "summary", "Analysis mode: summary, rolling, concentration, predict, montecarlo")
		windowSize  = flag.Int("window", 1000, "Rolling window size")
		tau         = flag.Uint64("tau", 1800, "Duration in slots (for prediction)")
		ethPrice    = flag.Float64("eth-price", 3500, "ETH price in USD")
		bridgeTVL   = flag.Float64("bridge-tvl", 500000000, "Bridge TVL in USD")
		successProb = flag.Float64("success-prob", 0.8, "Attack success probability")
		simulations = flag.Int("simulations", 10000, "Number of Monte Carlo simulations")
	)
	flag.Parse()

	// Load data
	bribes, err := loadBribesFromFile(*dataFile)
	if err != nil {
		log.Fatalf("Failed to load data: %v", err)
	}

	if len(bribes) == 0 {
		log.Fatal("No bribe data loaded")
	}

	fmt.Printf("Loaded %d slot bribes\n\n", len(bribes))

	stats := analysis.NewStatistics(bribes)

	switch *mode {
	case "summary":
		runSummaryAnalysis(stats)

	case "rolling":
		runRollingAnalysis(stats, *windowSize)

	case "concentration":
		runConcentrationAnalysis(stats, *windowSize)

	case "predict":
		runPrediction(stats, *tau, *ethPrice)

	case "montecarlo":
		runMonteCarloSimulation(bribes, *tau, *ethPrice, *bridgeTVL, *successProb, *simulations)

	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}
}

func runSummaryAnalysis(stats *analysis.Statistics) {
	fmt.Println("Statistical Summary")
	fmt.Println("===================")

	summary := stats.ComputeSummary()

	fmt.Printf("Count:        %d slots\n", summary.Count)
	fmt.Printf("Total:        %.6f ETH\n", summary.TotalETH)
	fmt.Printf("Mean:         %.6f ETH\n", summary.MeanETH)
	fmt.Printf("Median:       %.6f ETH\n", summary.MedianETH)
	fmt.Printf("Std Dev:      %.6f ETH\n", summary.StdDevETH)
	fmt.Printf("Min:          %.6f ETH\n", summary.MinETH)
	fmt.Printf("Max:          %.6f ETH\n", summary.MaxETH)
	fmt.Printf("25th pctl:    %.6f ETH\n", summary.P25ETH)
	fmt.Printf("75th pctl:    %.6f ETH\n", summary.P75ETH)
	fmt.Printf("95th pctl:    %.6f ETH\n", summary.P95ETH)
	fmt.Printf("99th pctl:    %.6f ETH\n", summary.P99ETH)
}

func runRollingAnalysis(stats *analysis.Statistics, windowSize int) {
	fmt.Printf("Rolling Statistics (window=%d)\n", windowSize)
	fmt.Println("===============================")

	rolling := stats.ComputeRollingStats(windowSize)

	if len(rolling) == 0 {
		fmt.Println("Not enough data for rolling analysis")
		return
	}

	// Print first 10 and last 10
	fmt.Println("\nFirst 10 windows:")
	for i := 0; i < 10 && i < len(rolling); i++ {
		r := rolling[i]
		fmt.Printf("Slot %d: mean=%.4f std=%.4f min=%.4f max=%.4f ETH\n",
			r.Slot, r.MeanETH, r.StdDevETH, r.MinETH, r.MaxETH)
	}

	if len(rolling) > 10 {
		fmt.Println("\nLast 10 windows:")
		for i := len(rolling) - 10; i < len(rolling); i++ {
			r := rolling[i]
			fmt.Printf("Slot %d: mean=%.4f std=%.4f min=%.4f max=%.4f ETH\n",
				r.Slot, r.MeanETH, r.StdDevETH, r.MinETH, r.MaxETH)
		}
	}
}

func runConcentrationAnalysis(stats *analysis.Statistics, windowSize int) {
	fmt.Printf("Builder Concentration Trends (window=%d)\n", windowSize)
	fmt.Println("=========================================")

	trends := stats.ComputeConcentrationTrends(windowSize)

	if len(trends) == 0 {
		fmt.Println("Not enough data for concentration analysis")
		return
	}

	// Print summary of trends
	fmt.Println("\nFirst 10 windows:")
	for i := 0; i < 10 && i < len(trends); i++ {
		t := trends[i]
		fmt.Printf("Slot %d: α(top3)=%.3f α(top5)=%.3f unique=%d HHI=%.3f\n",
			t.Slot, t.ConcentrationTop3, t.ConcentrationTop5, t.UniqueBuilders, t.HerfindahlIndex)
	}

	if len(trends) > 10 {
		fmt.Println("\nLast 10 windows:")
		for i := len(trends) - 10; i < len(trends); i++ {
			t := trends[i]
			fmt.Printf("Slot %d: α(top3)=%.3f α(top5)=%.3f unique=%d HHI=%.3f\n",
				t.Slot, t.ConcentrationTop3, t.ConcentrationTop5, t.UniqueBuilders, t.HerfindahlIndex)
		}
	}

	// Compute overall averages
	var avgTop3, avgTop5, avgHHI float64
	for _, t := range trends {
		avgTop3 += t.ConcentrationTop3
		avgTop5 += t.ConcentrationTop5
		avgHHI += t.HerfindahlIndex
	}
	n := float64(len(trends))

	fmt.Println("\nAverage Metrics:")
	fmt.Printf("Avg α(top3): %.3f\n", avgTop3/n)
	fmt.Printf("Avg α(top5): %.3f\n", avgTop5/n)
	fmt.Printf("Avg HHI:     %.3f\n", avgHHI/n)
}

func runPrediction(stats *analysis.Statistics, tau uint64, ethPrice float64) {
	fmt.Printf("Cost Prediction (τ=%d slots)\n", tau)
	fmt.Println("============================")

	// Use EMA with alpha=0.1
	predictedCost, err := stats.PredictFutureCost(tau, 0.1)
	if err != nil {
		log.Fatalf("Prediction failed: %v", err)
	}

	fmt.Printf("Predicted total cost: %.4f ETH\n", predictedCost)
	fmt.Printf("Predicted cost (USD): $%.2f\n", predictedCost*ethPrice)
	fmt.Printf("Average per slot:     %.6f ETH\n", predictedCost/float64(tau))
}

func runMonteCarloSimulation(bribes []model.SlotBribe, tau uint64, ethPrice, bridgeTVL, successProb float64, numSims int) {
	fmt.Printf("Monte Carlo Simulation (%d runs)\n", numSims)
	fmt.Println("=================================")

	// Compute actual censorship cost
	cost, err := model.CensorshipCost(bribes, tau)
	if err != nil {
		log.Fatalf("Failed to compute cost: %v", err)
	}

	weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))
	costETH, _ := new(big.Float).Quo(new(big.Float).SetInt(cost), weiPerEth).Float64()

	fmt.Printf("\nInput Parameters:\n")
	fmt.Printf("Censorship Cost:     %.4f ETH ($%.2f)\n", costETH, costETH*ethPrice)
	fmt.Printf("Bridge TVL:          $%.2f\n", bridgeTVL)
	fmt.Printf("Success Probability: %.2f%%\n", successProb*100)
	fmt.Printf("Simulations:         %d\n", numSims)
	fmt.Println()

	result := analysis.SimulateAttackOutcomes(costETH, bridgeTVL, ethPrice, successProb, numSims)
	analysis.PrintMonteCarloResult(result)

	// Breakeven analysis
	fmt.Println("\nBreakeven Analysis")
	fmt.Println("==================")
	breakeven := analysis.ComputeBreakevenAnalysis(costETH, ethPrice, successProb, bridgeTVL)
	fmt.Printf("Breakeven TVL:       $%.2f\n", breakeven.BreakevenTVL)
	fmt.Printf("Profit Margin:       %.2f%%\n", breakeven.ProfitMarginPercent)
}

func loadBribesFromFile(filename string) ([]model.SlotBribe, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var bribes []model.SlotBribe
	if err := json.Unmarshal(data, &bribes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return bribes, nil
}
