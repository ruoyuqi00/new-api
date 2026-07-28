package model

// selectChannelIndexByWeight applies the channel scheduler's weight rules.
// Positive weights are proportional. If every weight is zero, channels are
// treated equally so an unconfigured pool remains usable.
func selectChannelIndexByWeight(weights []int, randomInt func(int) int) int {
	if len(weights) == 0 {
		return -1
	}

	total := 0
	for _, weight := range weights {
		if weight > 0 {
			total += weight
		}
	}
	if total == 0 {
		return randomInt(len(weights))
	}

	draw := randomInt(total)
	for index, weight := range weights {
		if weight <= 0 {
			continue
		}
		if draw < weight {
			return index
		}
		draw -= weight
	}
	return len(weights) - 1
}
