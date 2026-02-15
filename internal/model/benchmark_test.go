package model

import (
	"math/big"
	"testing"
)

// BenchmarkCensorshipCost measures performance of cost computation
func BenchmarkCensorshipCost(b *testing.B) {
	// Create large dataset
	bribes := make([]SlotBribe, 100000)
	for i := 0; i < 100000; i++ {
		bribes[i] = SlotBribe{
			Slot:          uint64(i),
			ValueWei:      big.NewInt(int64(1e18 + i*1e15)),
			BuilderPubkey: "builder_1",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CensorshipCost(bribes, 100000)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCensorshipCostSmall tests small tau performance
func BenchmarkCensorshipCostSmall(b *testing.B) {
	bribes := make([]SlotBribe, 1000)
	for i := 0; i < 1000; i++ {
		bribes[i] = SlotBribe{
			Slot:          uint64(i),
			ValueWei:      big.NewInt(int64(1e18)),
			BuilderPubkey: "builder_1",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CensorshipCost(bribes, 100)
	}
}

// BenchmarkBuilderConcentration measures concentration computation performance
func BenchmarkBuilderConcentration(b *testing.B) {
	// Create dataset with realistic builder distribution
	bribes := make([]SlotBribe, 100000)
	builders := []string{"builder_1", "builder_2", "builder_3", "builder_4", "builder_5"}

	for i := 0; i < 100000; i++ {
		bribes[i] = SlotBribe{
			Slot:          uint64(i),
			ValueWei:      big.NewInt(int64(1e18)),
			BuilderPubkey: builders[i%len(builders)],
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := ComputeBuilderConcentration(bribes, 3)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEffectiveCensorshipCost tests full pipeline performance
func BenchmarkEffectiveCensorshipCost(b *testing.B) {
	bribes := make([]SlotBribe, 10000)
	builders := []string{"builder_1", "builder_2", "builder_3", "builder_4", "builder_5"}

	for i := 0; i < 10000; i++ {
		bribes[i] = SlotBribe{
			Slot:          uint64(i),
			ValueWei:      big.NewInt(int64(1e18 + i*1e15)),
			BuilderPubkey: builders[i%len(builders)],
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := EffectiveCensorshipCost(bribes, 10000, 3)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFindBreakevenTVL tests breakeven calculation performance
func BenchmarkFindBreakevenTVL(b *testing.B) {
	bribes := make([]SlotBribe, 1000)
	for i := 0; i < 1000; i++ {
		bribes[i] = SlotBribe{
			Slot:          uint64(i),
			ValueWei:      big.NewInt(int64(2e18)),
			BuilderPubkey: "builder_1",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := FindBreakevenTVL(bribes, 0.8, 1000, 3)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBigIntArithmetic tests big.Int overhead vs uint64
func BenchmarkBigIntArithmetic(b *testing.B) {
	values := make([]*big.Int, 100000)
	for i := 0; i < 100000; i++ {
		values[i] = big.NewInt(int64(1e18))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := new(big.Int)
		for _, v := range values {
			total.Add(total, v)
		}
	}
}

// BenchmarkUint64Arithmetic compares with native uint64 (baseline)
func BenchmarkUint64Arithmetic(b *testing.B) {
	values := make([]uint64, 100000)
	for i := 0; i < 100000; i++ {
		values[i] = 1e18
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var total uint64
		for _, v := range values {
			total += v
		}
	}
}
