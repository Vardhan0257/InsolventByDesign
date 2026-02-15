package main

import (
	"fmt"

	"insolventbydesign/internal/model"
)

func main() {
	bribes := []model.SlotBribe{
		{Slot: 1, Cost: 1.5},
		{Slot: 2, Cost: 2.0},
		{Slot: 3, Cost: 2.5},
	}

	tau := uint64(2)
	cost := model.CensorshipCost(bribes, tau)

	fmt.Printf("Censorship cost for tau=%d slots: %.2f\n", tau, cost)
}
