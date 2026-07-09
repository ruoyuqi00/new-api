package common

import (
	"math"
	"strconv"
)

type QuotaClamp struct {
	Op       string `json:"op,omitempty"`
	Kind     string `json:"kind"`
	Original string `json:"original"`
	Clamped  int    `json:"clamped"`
}

func (q *QuotaClamp) WithOp(op string) *QuotaClamp {
	if q == nil {
		return nil
	}
	copy := *q
	copy.Op = op
	return &copy
}

func (q *QuotaClamp) AuditMap() map[string]interface{} {
	if q == nil {
		return nil
	}
	result := map[string]interface{}{
		"kind":     q.Kind,
		"original": q.Original,
		"clamped":  q.Clamped,
	}
	if q.Op != "" {
		result["op"] = q.Op
	}
	return result
}

// QuotaFromFloat converts a computed quota value to int with saturation.
// Quota products can include user-controlled multipliers such as image counts,
// video seconds, and resolution ratios. Oversized products must not wrap around
// into negative charges. The bound is int32 because quota columns are 32-bit in
// the database schema.
func QuotaFromFloat(value float64) int {
	quota, _ := QuotaFromFloatChecked(value)
	return quota
}

func QuotaFromFloatChecked(value float64) (int, *QuotaClamp) {
	if math.IsNaN(value) {
		return 0, quotaClamp("nan", value, 0)
	}
	if value >= math.MaxInt32 {
		return math.MaxInt32, quotaClamp("overflow", value, math.MaxInt32)
	}
	if value <= math.MinInt32 {
		return math.MinInt32, quotaClamp("underflow", value, math.MinInt32)
	}
	return int(value), nil
}

func quotaClamp(kind string, original float64, clamped int) *QuotaClamp {
	return &QuotaClamp{
		Kind:     kind,
		Original: strconv.FormatFloat(original, 'g', -1, 64),
		Clamped:  clamped,
	}
}
