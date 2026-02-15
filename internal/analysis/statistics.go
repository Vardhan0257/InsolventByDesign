package analysis

import (
	"fmt"
	"math"
	"math/big"
	"sort"

	"insolventbydesign/internal/model"
)

// Statistics provides statistical analysis of censorship data.
type Statistics struct {
	bribes []model.SlotBribe
}

// NewStatistics creates a new statistics analyzer.
func NewStatistics(bribes []model.SlotBribe) *Statistics {
	return &Statistics{bribes: bribes}
}

// Summary contains statistical summary of bribe data.
type Summary struct {
	Count     int
	MeanETH   float64
	MedianETH float64
	StdDevETH float64
	MinETH    float64
	MaxETH    float64
	P25ETH    float64
	P75ETH    float64
	P95ETH    float64
	P99ETH    float64
	TotalETH  float64
}

// ComputeSummary computes comprehensive statistics.
func (s *Statistics) ComputeSummary() Summary {
	if len(s.bribes) == 0 {
		return Summary{}
	}

	valuesETH := make([]float64, len(s.bribes))
	weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))

	for i, bribe := range s.bribes {
		if bribe.ValueWei != nil {
			ethVal := new(big.Float).Quo(new(big.Float).SetInt(bribe.ValueWei), weiPerEth)
			valuesETH[i], _ = ethVal.Float64()
		}
	}

	sort.Float64s(valuesETH)

	summary := Summary{
		Count:     len(valuesETH),
		MinETH:    valuesETH[0],
		MaxETH:    valuesETH[len(valuesETH)-1],
		MedianETH: percentile(valuesETH, 50),
		P25ETH:    percentile(valuesETH, 25),
		P75ETH:    percentile(valuesETH, 75),
		P95ETH:    percentile(valuesETH, 95),
		P99ETH:    percentile(valuesETH, 99),
	}

	// Compute mean
	var sum float64
	for _, v := range valuesETH {
		sum += v
	}
	summary.MeanETH = sum / float64(len(valuesETH))
	summary.TotalETH = sum

	// Compute standard deviation
	var variance float64
	for _, v := range valuesETH {
		diff := v - summary.MeanETH
		variance += diff * diff
	}
	summary.StdDevETH = math.Sqrt(variance / float64(len(valuesETH)))

	return summary
}

// RollingStatistics computes rolling window statistics.
type RollingStatistics struct {
	Slot      uint64
	MeanETH   float64
	StdDevETH float64
	MaxETH    float64
	MinETH    float64
}

// ComputeRollingStats computes statistics over sliding windows.
func (s *Statistics) ComputeRollingStats(windowSize int) []RollingStatistics {
	if len(s.bribes) < windowSize {
		return nil
	}

	results := make([]RollingStatistics, 0, len(s.bribes)-windowSize+1)
	weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))

	for i := windowSize - 1; i < len(s.bribes); i++ {
		window := s.bribes[i-windowSize+1 : i+1]

		values := make([]float64, windowSize)
		for j, bribe := range window {
			if bribe.ValueWei != nil {
				ethVal := new(big.Float).Quo(new(big.Float).SetInt(bribe.ValueWei), weiPerEth)
				values[j], _ = ethVal.Float64()
			}
		}

		stat := RollingStatistics{
			Slot:   s.bribes[i].Slot,
			MaxETH: maxFloat64(values),
			MinETH: minFloat64(values),
		}

		// Mean
		var sum float64
		for _, v := range values {
			sum += v
		}
		stat.MeanETH = sum / float64(len(values))

		// StdDev
		var variance float64
		for _, v := range values {
			diff := v - stat.MeanETH
			variance += diff * diff
		}
		stat.StdDevETH = math.Sqrt(variance / float64(len(values)))

		results = append(results, stat)
	}

	return results
}

// ConcentrationTrend tracks builder concentration over time.
type ConcentrationTrend struct {
	Slot              uint64
	ConcentrationTop3 float64
	ConcentrationTop5 float64
	UniqueBuilders    int
	HerfindahlIndex   float64
}

// ComputeConcentrationTrends computes rolling concentration metrics.
func (s *Statistics) ComputeConcentrationTrends(windowSize int) []ConcentrationTrend {
	if len(s.bribes) < windowSize {
		return nil
	}

	results := make([]ConcentrationTrend, 0, len(s.bribes)-windowSize+1)

	for i := windowSize - 1; i < len(s.bribes); i++ {
		window := s.bribes[i-windowSize+1 : i+1]

		alpha3, _, _ := model.ComputeBuilderConcentration(window, 3)
		alpha5, _, _ := model.ComputeBuilderConcentration(window, 5)

		// Count unique builders
		builderSet := make(map[string]bool)
		builderCounts := make(map[string]int)
		for _, bribe := range window {
			builderSet[bribe.BuilderPubkey] = true
			builderCounts[bribe.BuilderPubkey]++
		}

		// Herfindahl-Hirschman Index
		var hhi float64
		for _, count := range builderCounts {
			share := float64(count) / float64(len(window))
			hhi += share * share
		}

		results = append(results, ConcentrationTrend{
			Slot:              s.bribes[i].Slot,
			ConcentrationTop3: alpha3,
			ConcentrationTop5: alpha5,
			UniqueBuilders:    len(builderSet),
			HerfindahlIndex:   hhi,
		})
	}

	return results
}

// PredictFutureCost uses exponential moving average for simple prediction.
func (s *Statistics) PredictFutureCost(tau uint64, alpha float64) (float64, error) {
	if len(s.bribes) == 0 {
		return 0, fmt.Errorf("no data available")
	}

	weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))

	// Start with first value
	firstVal := new(big.Float).Quo(new(big.Float).SetInt(s.bribes[0].ValueWei), weiPerEth)
	ema, _ := firstVal.Float64()

	// Compute EMA
	for i := 1; i < len(s.bribes); i++ {
		if s.bribes[i].ValueWei != nil {
			ethVal := new(big.Float).Quo(new(big.Float).SetInt(s.bribes[i].ValueWei), weiPerEth)
			val, _ := ethVal.Float64()
			ema = alpha*val + (1-alpha)*ema
		}
	}

	// Predict future cost as EMA * tau
	return ema * float64(tau), nil
}

// Helper functions

func percentile(sortedData []float64, p float64) float64 {
	if len(sortedData) == 0 {
		return 0
	}
	index := (p / 100.0) * float64(len(sortedData)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sortedData[lower]
	}

	weight := index - float64(lower)
	return sortedData[lower]*(1-weight) + sortedData[upper]*weight
}

func maxFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func minFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}
