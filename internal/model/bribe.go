package model

// SlotBribe represents the minimum cost required
// to exclude a transaction from a single slot.
type SlotBribe struct {
	Slot uint64
	Cost float64
}

// CensorshipCost computes the total cost required
// to censor a transaction for tau consecutive slots.
func CensorshipCost(bribes []SlotBribe, tau uint64) float64 {
	var total float64
	var counted uint64

	for _, b := range bribes {
		if counted >= tau {
			break
		}
		total += b.Cost
		counted++
	}

	return total
}
