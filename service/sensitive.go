package service

import (
	"errors"
	"strings"
	"sync"
	"unicode"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
	goahocorasick "github.com/anknown/ahocorasick"
	"golang.org/x/text/unicode/norm"
)

type sensitiveMatcherSet struct {
	normalized *goahocorasick.Machine
	compact    *goahocorasick.Machine
}

var sensitiveMatcherCache sync.Map

func CheckSensitiveMessages(messages []dto.Message) ([]string, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	for _, message := range messages {
		arrayContent := message.ParseContent()
		for _, m := range arrayContent {
			if m.Type == "image_url" {
				// TODO: check image url
				continue
			}
			// 检查 text 是否为空
			if m.Text == "" {
				continue
			}
			if ok, words := SensitiveWordContains(m.Text); ok {
				return words, errors.New("sensitive words detected")
			}
		}
	}
	return nil, nil
}

func CheckSensitiveText(text string) (bool, []string) {
	return SensitiveWordContains(text)
}

// SensitiveWordContains 是否包含敏感词，返回是否包含敏感词和敏感词列表
func SensitiveWordContains(text string) (bool, []string) {
	if len(setting.SensitiveWords) == 0 {
		return false, nil
	}
	if len(text) == 0 {
		return false, nil
	}
	matchers := getSensitiveMatcherSet(setting.SensitiveWords)
	if matchers == nil {
		return false, nil
	}
	normalizedText, compactText := normalizeSensitiveValues(text)
	if matched, words := searchSensitiveMatcher(normalizedText, matchers.normalized); matched {
		return true, words
	}
	return searchSensitiveMatcher(compactText, matchers.compact)
}

func getSensitiveMatcherSet(words []string) *sensitiveMatcherSet {
	key := acKey(words)
	if key == "" {
		return nil
	}
	if cached, ok := sensitiveMatcherCache.Load(key); ok {
		return cached.(*sensitiveMatcherSet)
	}

	matchers := &sensitiveMatcherSet{
		normalized: InitAc(normalizeSensitiveDictionary(words, false)),
		compact:    InitAc(normalizeSensitiveDictionary(words, true)),
	}
	if cached, loaded := sensitiveMatcherCache.LoadOrStore(key, matchers); loaded {
		return cached.(*sensitiveMatcherSet)
	}
	return matchers
}

func searchSensitiveMatcher(text string, matcher *goahocorasick.Machine) (bool, []string) {
	if text == "" || matcher == nil {
		return false, nil
	}
	hits := matcher.MultiPatternSearch([]rune(text), true)
	if len(hits) == 0 {
		return false, nil
	}
	return true, []string{string(hits[0].Word)}
}

func normalizeSensitiveDictionary(words []string, compact bool) []string {
	normalized := make([]string, 0, len(words))
	for _, word := range words {
		word = normalizeSensitiveValue(word, compact)
		if strings.TrimSpace(word) != "" {
			normalized = append(normalized, word)
		}
	}
	return normalized
}

func normalizeSensitiveValue(value string, compact bool) string {
	normalized, compactValue := normalizeSensitiveValues(value)
	if compact {
		return compactValue
	}
	return normalized
}

func normalizeSensitiveValues(value string) (string, string) {
	value = norm.NFKD.String(strings.ToLower(value))
	var normalized strings.Builder
	var compact strings.Builder
	normalized.Grow(len(value))
	compact.Grow(len(value))
	for _, r := range value {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if unicode.IsControl(r) {
			normalized.WriteByte(' ')
			continue
		}
		normalized.WriteRune(r)
		if !unicode.IsSpace(r) && !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			compact.WriteRune(r)
		}
	}
	return normalized.String(), compact.String()
}

// SensitiveWordReplace 敏感词替换，返回是否包含敏感词和替换后的文本
func SensitiveWordReplace(text string, returnImmediately bool) (bool, []string, string) {
	if len(setting.SensitiveWords) == 0 {
		return false, nil, text
	}
	checkText := strings.ToLower(text)
	m := getOrBuildAC(setting.SensitiveWords)
	hits := m.MultiPatternSearch([]rune(checkText), returnImmediately)
	if len(hits) > 0 {
		words := make([]string, 0, len(hits))
		var builder strings.Builder
		builder.Grow(len(text))
		lastPos := 0

		for _, hit := range hits {
			pos := hit.Pos
			word := string(hit.Word)
			builder.WriteString(text[lastPos:pos])
			builder.WriteString("**###**")
			lastPos = pos + len(word)
			words = append(words, word)
		}
		builder.WriteString(text[lastPos:])
		return true, words, builder.String()
	}
	return false, nil, text
}
