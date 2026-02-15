package model

import "testing"

func TestCensorshipCost(t *testing.T) {
	bribes := []SlotBribe{
		{Slot: 1, Cost: 1.0},
		{Slot: 2, Cost: 2.0},
		{Slot: 3, Cost: 3.0},
	}

	cost := CensorshipCost(bribes, 2)

	if cost != 3.0 {
		t.Fatalf("expected cost 3.0, got %f", cost)
	}
}
