package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
)

func TestSensitiveWordContainsNormalizesObfuscatedInput(t *testing.T) {
	previousWords := append([]string(nil), setting.SensitiveWords...)
	setting.SensitiveWords = []string{"danger", "blocked phrase"}
	t.Cleanup(func() { setting.SensitiveWords = previousWords })

	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "case insensitive", text: "DANGER", want: true},
		{name: "full width compatibility characters", text: "ＤＡＮＧＥＲ", want: true},
		{name: "zero width insertion", text: "dan\u200bger", want: true},
		{name: "combining mark insertion", text: "da\u0301nger", want: true},
		{name: "punctuation and whitespace insertion", text: "d.a n-g/e_r", want: true},
		{name: "phrase separator obfuscation", text: "blocked...phrase", want: true},
		{name: "phrase separator omission", text: "blockedphrase", want: true},
		{name: "alphanumeric insertion remains distinct", text: "danxger", want: false},
		{name: "unrelated input", text: "ordinary request", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, _ := SensitiveWordContains(tt.text)
			assert.Equal(t, tt.want, matched)
		})
	}
}

func TestSensitiveWordContainsNormalizesDictionaryEntries(t *testing.T) {
	previousWords := append([]string(nil), setting.SensitiveWords...)
	setting.SensitiveWords = []string{"Ｆｌａｇｇｅｄ"}
	t.Cleanup(func() { setting.SensitiveWords = previousWords })

	matched, _ := SensitiveWordContains("flagged")
	assert.True(t, matched)
}
