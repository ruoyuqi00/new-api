package common

import "math"

// QuotaFromFloat converts a computed quota value to int with saturation.
// Quota products can include user-controlled multipliers such as image counts,
// video seconds, and resolution ratios. Oversized products must not wrap around
// into negative charges. The bound is int32 because quota columns are 32-bit in
// the database schema.
func QuotaFromFloat(value float64) int {
	if math.IsNaN(value) {
		return 0
	}
	if value >= math.MaxInt32 {
		return math.MaxInt32
	}
	if value <= math.MinInt32 {
		return math.MinInt32
	}
	return int(value)
}
