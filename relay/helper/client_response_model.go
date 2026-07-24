package helper

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func NormalizeClientResponseModelJSON(info *relaycommon.RelayInfo, data []byte) ([]byte, bool, error) {
	if info == nil || info.ChannelMeta == nil || !info.IsModelMapped || strings.TrimSpace(info.OriginModelName) == "" || !gjson.ValidBytes(data) {
		return data, false, nil
	}

	modelName := info.ClientResponseModelName()
	result := data
	changed := false
	for _, path := range []string{"model", "response.model"} {
		model := gjson.GetBytes(result, path)
		if !model.Exists() || model.Type != gjson.String || model.String() == modelName {
			continue
		}

		updated, err := sjson.SetBytes(result, path, modelName)
		if err != nil {
			return data, false, err
		}
		result = updated
		changed = true
	}

	return result, changed, nil
}
