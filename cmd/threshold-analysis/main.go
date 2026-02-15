package main

import (
	"fmt"
	"log"
	"math/big"
	"strings"

	"insolventbydesign/internal/model"
	"insolventbydesign/internal/relay"
)

// ThresholdScenario defines a specific attack scenario to evaluate.
type ThresholdScenario struct {
	Name        string
	Tau         uint64  // Censorship duration in slots
	TopK        int     // Number of top builders in cartel
	SuccessProb float64 // Assumed success probability
}

func main() {
	fmt.Println("=======================================================")
	fmt.Println("INSOLVENTBYDESIGN — THRESHOLD DISCOVERY")
	fmt.Println("=======================================================")
	fmt.Println()

	// Load real relay data
	dataDir := "data/relay_raw"
	fmt.Printf("Loading relay data from: %s\n", dataDir)

	bribes, err := relay.ParseRelayDirectory(dataDir)
	if err != nil {
		log.Fatalf("Failed to load relay data: %v", err)
	}

	if len(bribes) == 0 {
		log.Fatal("No relay data found. Please fetch relay data first.")
	}

	fmt.Printf("✓ Loaded %d slot bribes\n", len(bribes))
	fmt.Println()

	// Analyze builder concentration
	diversity := model.GetBuilderDiversity(bribes)
	fmt.Printf("Builder diversity: %d unique builders\n", diversity)

	// Show top builders
	topBuilders, err := model.GetTopBuilders(bribes, 5)
	if err == nil && len(topBuilders) > 0 {
		fmt.Println("\nTop 5 builders by block count:")
		for i, builder := range topBuilders {
			pct := float64(builder.BlockCount) / float64(len(bribes)) * 100
			pubkey := builder.BuilderPubkey
			if len(pubkey) > 20 {
				pubkey = pubkey[:10] + "..." + pubkey[len(pubkey)-6:]
			}
			fmt.Printf("  %d. %s: %d blocks (%.1f%%)\n", i+1, pubkey, builder.BlockCount, pct)
		}
	}
	fmt.Println()

	// Define scenarios to evaluate
	scenarios := []ThresholdScenario{
		{
			Name:        "Conservative (τ=10, k=3, p=0.1)",
			Tau:         10,
			TopK:        3,
			SuccessProb: 0.1,
		},
		{
			Name:        "Moderate (τ=10, k=3, p=0.5)",
			Tau:         10,
			TopK:        3,
			SuccessProb: 0.5,
		},
		{
			Name:        "Aggressive (τ=10, k=3, p=0.9)",
			Tau:         10,
			TopK:        3,
			SuccessProb: 0.9,
		},
		{
			Name:        "Extended (τ=50, k=5, p=0.5)",
			Tau:         50,
			TopK:        5,
			SuccessProb: 0.5,
		},
	}

	fmt.Println("=======================================================")
	fmt.Println("THRESHOLD ANALYSIS")
	fmt.Println("=======================================================")
	fmt.Println()

	for _, scenario := range scenarios {
		if err := analyzeScenario(bribes, scenario); err != nil {
			fmt.Printf("⚠ Scenario '%s' failed: %v\n\n", scenario.Name, err)
			continue
		}
	}

	fmt.Println("=======================================================")
	fmt.Println("CRITICAL DISCLAIMER")
	fmt.Println("=======================================================")
	fmt.Println()
	fmt.Println("These thresholds are computed under EXPLICIT ASSUMPTIONS:")
	fmt.Println("  - Success probability p is ASSUMED, not derived")
	fmt.Println("  - Bridge defense mechanisms are NOT modeled")
	fmt.Println("  - Inclusion lists (EIP-7547) are NOT considered")
	fmt.Println("  - Social/legal consequences are NOT factored")
	fmt.Println()
	fmt.Println("This analysis demonstrates economic BOUNDS, not attack")
	fmt.Println("feasibility. Real security requires defense in depth.")
	fmt.Println()
}

func analyzeScenario(bribes []model.SlotBribe, scenario ThresholdScenario) error {
	fmt.Printf("Scenario: %s\n", scenario.Name)
	fmt.Println(strings.Repeat("-", 55))

	// Check if we have enough data
	if uint64(len(bribes)) < scenario.Tau {
		return fmt.Errorf("insufficient data (have %d slots, need %d)", len(bribes), scenario.Tau)
	}

	// Compute raw censorship cost
	cc, err := model.CensorshipCost(bribes, scenario.Tau)
	if err != nil {
		return fmt.Errorf("failed to compute censorship cost: %w", err)
	}

	// Compute effective censorship cost with concentration
	ccEff, alpha, err := model.EffectiveCensorshipCost(bribes, scenario.Tau, scenario.TopK)
	if err != nil {
		return fmt.Errorf("failed to compute effective cost: %w", err)
	}

	// Compute breakeven TVL threshold
	breakeven, _, err := model.FindBreakevenTVL(bribes, scenario.SuccessProb, scenario.Tau, scenario.TopK)
	if err != nil {
		return fmt.Errorf("failed to compute breakeven: %w", err)
	}

	// Convert to ETH for readability
	weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))

	ccEth := new(big.Float).Quo(new(big.Float).SetInt(cc), weiPerEth)
	ccEffEth := new(big.Float).Quo(ccEff, weiPerEth)
	breakevenEth := new(big.Float).Quo(breakeven, weiPerEth)

	// Convert to USD (assuming $3000/ETH for reference)
	ethToUSD := 3000.0
	ccEffUSD := new(big.Float).Mul(ccEffEth, big.NewFloat(ethToUSD))
	breakevenUSD := new(big.Float).Mul(breakevenEth, big.NewFloat(ethToUSD))

	fmt.Printf("  Censorship duration (τ):     %d slots\n", scenario.Tau)
	fmt.Printf("  Cartel size (k):              %d builders\n", scenario.TopK)
	fmt.Printf("  Builder concentration (α):    %.3f\n", alpha)
	fmt.Printf("  Assumed success prob (p):     %.2f\n", scenario.SuccessProb)
	fmt.Println()
	fmt.Printf("  Raw censorship cost (C_c):    %s ETH\n", formatFloat(ccEth))
	fmt.Printf("  Effective cost (C_c^eff):     %s ETH (~$%s)\n",
		formatFloat(ccEffEth), formatFloat(ccEffUSD))
	fmt.Println()
	fmt.Printf("  BREAKEVEN TVL (V*):           %s ETH\n", formatFloat(breakevenEth))
	fmt.Printf("                                ~$%s\n", formatFloat(breakevenUSD))
	fmt.Println()

	// Show profitability at different TVL levels
	testTVLs := []float64{10_000_000, 50_000_000, 100_000_000, 500_000_000, 1_000_000_000}
	fmt.Println("  Profit at different TVL levels (USD):")

	for _, tvlUSD := range testTVLs {
		tvlETH := tvlUSD / ethToUSD
		tvlWei := new(big.Float).Mul(big.NewFloat(tvlETH), weiPerEth)

		params := model.ProfitParams{
			BridgeTVL:          tvlWei,
			SuccessProbability: scenario.SuccessProb,
			Tau:                scenario.Tau,
			TopK:               scenario.TopK,
		}

		result, err := model.AttackerProfit(bribes, params)
		if err != nil {
			continue
		}

		profitETH := new(big.Float).Quo(result.Profit, weiPerEth)
		profitUSD := new(big.Float).Mul(profitETH, big.NewFloat(ethToUSD))

		profitSign := " "
		if result.Profit.Sign() > 0 {
			profitSign = "✓"
		} else if result.Profit.Sign() < 0 {
			profitSign = "✗"
		}

		fmt.Printf("    %s TVL=$%s → Profit=$%s\n",
			profitSign, formatMillion(tvlUSD), formatFloat(profitUSD))
	}

	fmt.Println()
	return nil
}

func formatFloat(f *big.Float) string {
	val, _ := f.Float64()
	if val >= 1e9 {
		return fmt.Sprintf("%.2fB", val/1e9)
	} else if val >= 1e6 {
		return fmt.Sprintf("%.2fM", val/1e6)
	} else if val >= 1e3 {
		return fmt.Sprintf("%.2fK", val/1e3)
	}
	return fmt.Sprintf("%.2f", val)
}

func formatMillion(val float64) string {
	if val >= 1e9 {
		return fmt.Sprintf("%.1fB", val/1e9)
	} else if val >= 1e6 {
		return fmt.Sprintf("%.0fM", val/1e6)
	}
	return fmt.Sprintf("%.0f", val)
}
