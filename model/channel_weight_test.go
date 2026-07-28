package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectChannelIndexByWeight(t *testing.T) {
	tests := []struct {
		name    string
		weights []int
		draw    int
		want    int
	}{
		{name: "empty", weights: nil, want: -1},
		{name: "all zero uses equal slots", weights: []int{0, 0, 0}, draw: 2, want: 2},
		{name: "first weighted boundary", weights: []int{70, 30}, draw: 69, want: 0},
		{name: "second weighted boundary", weights: []int{70, 30}, draw: 70, want: 1},
		{name: "zero weight is skipped in mixed pool", weights: []int{0, 5}, draw: 0, want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			got := selectChannelIndexByWeight(test.weights, func(max int) int {
				calls++
				require.Greater(t, max, 0)
				require.Less(t, test.draw, max)
				return test.draw
			})
			assert.Equal(t, test.want, got)
			if len(test.weights) == 0 {
				assert.Zero(t, calls)
			} else {
				assert.Equal(t, 1, calls)
			}
		})
	}
}
