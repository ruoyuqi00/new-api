package helper

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const actualResponseModelMaxLength = 100

func CaptureActualResponseModelJSON(info *relaycommon.RelayInfo, data []byte) {
	if info == nil {
		return
	}
	if info.ForwardedModelName == "" && info.ChannelMeta != nil {
		info.ForwardedModelName = strings.TrimSpace(info.UpstreamModelName)
	}
	if info.ActualResponseModel != "" || !gjson.ValidBytes(data) {
		return
	}

	for _, path := range []string{"response.model", "model", "message.model", "session.model"} {
		value := gjson.GetBytes(data, path)
		if !value.Exists() || value.Type != gjson.String {
			continue
		}
		modelName := strings.TrimSpace(value.String())
		if modelName == "" {
			continue
		}
		runes := []rune(modelName)
		if len(runes) > actualResponseModelMaxLength {
			modelName = string(runes[:actualResponseModelMaxLength])
		}
		info.ActualResponseModel = modelName
		return
	}
}

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
