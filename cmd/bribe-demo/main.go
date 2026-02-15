package main

import (
	"fmt"
	"log"
	"math/big"

	"insolventbydesign/internal/model"
)

func main() {
	bribes := []model.SlotBribe{
		{Slot: 1, ValueWei: big.NewInt(1500000000000000000)}, // 1.5 ETH
		{Slot: 2, ValueWei: big.NewInt(2000000000000000000)}, // 2.0 ETH
		{Slot: 3, ValueWei: big.NewInt(2500000000000000000)}, // 2.5 ETH
	}

	tau := uint64(2)
	cost, err := model.CensorshipCost(bribes, tau)
	if err != nil {
		log.Fatalf("CensorshipCost failed: %v", err)
	}

	// Convert to ETH for display
	weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))
	costEth := new(big.Float).Quo(new(big.Float).SetInt(cost), weiPerEth)

	fmt.Printf("Censorship cost for tau=%d slots: %s ETH (exact wei: %s)\n", tau, costEth.Text('f', 2), cost.String())
}
