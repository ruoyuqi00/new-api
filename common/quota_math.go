package common

import (
	"fmt"
	"math"
	"strconv"
)

type QuotaClamp struct {
	Op       string `json:"op,omitempty"`
	Kind     string `json:"kind"`
	Original string `json:"original"`
	Clamped  int    `json:"clamped"`
}

func (q *QuotaClamp) Error() string {
	if q == nil {
		return ""
	}
	if q.Op == "" {
		return fmt.Sprintf("quota conversion %s: original=%s, clamped=%d", q.Kind, q.Original, q.Clamped)
	}
	return fmt.Sprintf("quota conversion (%s) %s: original=%s, clamped=%d", q.Op, q.Kind, q.Original, q.Clamped)
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
	return quotaChecked(value)
}

func quotaChecked(value float64) (int, *QuotaClamp) {
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

// QuotaFromFloatStrict rejects values that cannot be represented by the
// int32-backed quota columns. Pre-consume paths must fail before deducting an
// ambiguous or saturated amount.
func QuotaFromFloatStrict(value float64) (int, error) {
	quota, clamp := QuotaFromFloatChecked(value)
	if clamp != nil {
		return 0, clamp
	}
	return quota, nil
}

func QuotaRound(value float64) int {
	quota, _ := QuotaRoundChecked(value)
	return quota
}

func QuotaRoundChecked(value float64) (int, *QuotaClamp) {
	return quotaChecked(math.Round(value))
}

func QuotaRoundStrict(value float64) (int, error) {
	quota, clamp := QuotaRoundChecked(value)
	if clamp != nil {
		return 0, clamp
	}
	return quota, nil
}

func quotaClamp(kind string, original float64, clamped int) *QuotaClamp {
	return &QuotaClamp{
		Kind:     kind,
		Original: strconv.FormatFloat(original, 'g', -1, 64),
		Clamped:  clamped,
	}
}
