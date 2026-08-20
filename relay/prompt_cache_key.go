package relay

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func injectPromptCacheKey(jsonData []byte, promptCacheKey string) ([]byte, error) {
	if strings.TrimSpace(promptCacheKey) == "" {
		return jsonData, nil
	}
	existing := gjson.GetBytes(jsonData, "prompt_cache_key")
	if existing.Exists() && strings.TrimSpace(existing.String()) != "" {
		return jsonData, nil
	}
	return sjson.SetBytes(jsonData, "prompt_cache_key", promptCacheKey)
}
