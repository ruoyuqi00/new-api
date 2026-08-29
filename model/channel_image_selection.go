package model

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

// ImageSelectionRequirements carries only the request fields needed before a
// channel is selected. It intentionally does not alter billing or relay data.
type ImageSelectionRequirements struct {
	Size        string
	AspectRatio string
}

func (requirements ImageSelectionRequirements) RequiresNonSquare() bool {
	if width, height, ok := parseImageDimensions(requirements.Size); ok && width != height {
		return true
	}
	ratio := strings.TrimSpace(strings.ToLower(requirements.AspectRatio))
	if ratio == "" || ratio == "auto" || ratio == "1:1" {
		return false
	}
	parts := strings.Split(ratio, ":")
	if len(parts) != 2 {
		return false
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	return widthErr == nil && heightErr == nil && width > 0 && height > 0 && width != height
}

// ChannelSupportsImageRequest filters only known-incompatible channels. An
// unconfigured channel remains eligible, preserving existing price and weight
// ordering while allowing admins to opt into stricter modes through settings.
func ChannelSupportsImageRequest(channel *Channel, modelName string, requirements ImageSelectionRequirements) bool {
	if channel == nil || !requirements.RequiresNonSquare() {
		return true
	}

	settings := channel.GetOtherSettings()
	switch strings.ToLower(strings.TrimSpace(settings.ImageDimensionSupport)) {
	case "square", "fixed_square", "fixed-square", "pending", "unknown":
		return false
	case "any", "custom", "ratio", "aspect_ratio", "aspect-ratio":
		return true
	}

	// OpenAI-compatible JSON requests are protected by the relay's image
	// dimension restoration after channel overrides are applied.
	if channel.Type == constant.ChannelTypeOpenAI {
		return true
	}
	return !channelParamOverrideForcesSquare(channel, modelName)
}

type imageParamOverrideOperation struct {
	Path       string                        `json:"path"`
	Mode       string                        `json:"mode"`
	Value      interface{}                   `json:"value"`
	KeepOrigin bool                          `json:"keep_origin"`
	Conditions []imageParamOverrideCondition `json:"conditions"`
	Logic      string                        `json:"logic"`
}

type imageParamOverrideCondition struct {
	Path   string      `json:"path"`
	Mode   string      `json:"mode"`
	Value  interface{} `json:"value"`
	Invert bool        `json:"invert"`
}

func channelParamOverrideForcesSquare(channel *Channel, modelName string) bool {
	if channel == nil || channel.ParamOverride == nil || strings.TrimSpace(*channel.ParamOverride) == "" {
		return false
	}

	override := channel.GetParamOverride()
	if imageSizeIsSquare(override["size"]) {
		return true
	}

	var operations []imageParamOverrideOperation
	if raw, ok := override["operations"]; ok {
		encoded, err := common.Marshal(raw)
		if err != nil || common.Unmarshal(encoded, &operations) != nil {
			return false
		}
	}
	for _, operation := range operations {
		if !strings.EqualFold(strings.TrimSpace(operation.Mode), "set") ||
			!strings.EqualFold(strings.TrimSpace(operation.Path), "size") ||
			operation.KeepOrigin || !imageSizeIsSquare(operation.Value) {
			continue
		}
		if imageOverrideConditionsMatch(operation.Conditions, operation.Logic, modelName) {
			return true
		}
	}
	return false
}

func imageOverrideConditionsMatch(conditions []imageParamOverrideCondition, logic string, modelName string) bool {
	if len(conditions) == 0 {
		return true
	}
	results := make([]bool, 0, len(conditions))
	for _, condition := range conditions {
		path := strings.ToLower(strings.TrimSpace(condition.Path))
		expected, ok := condition.Value.(string)
		if !ok {
			results = append(results, false)
			continue
		}
		var result bool
		switch path {
		case "model", "original_model":
			switch strings.ToLower(strings.TrimSpace(condition.Mode)) {
			case "", "full":
				result = expected == modelName
			case "prefix":
				result = strings.HasPrefix(modelName, expected)
			case "suffix":
				result = strings.HasSuffix(modelName, expected)
			case "contains":
				result = strings.Contains(modelName, expected)
			}
		default:
			// Unknown conditions may apply at runtime, so keep the channel out
			// of a non-square request rather than risk a square override.
			result = true
		}
		if condition.Invert {
			result = !result
		}
		results = append(results, result)
	}
	if strings.EqualFold(strings.TrimSpace(logic), "AND") {
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	}
	for _, result := range results {
		if result {
			return true
		}
	}
	return false
}

func imageSizeIsSquare(value interface{}) bool {
	size, ok := value.(string)
	if !ok {
		return false
	}
	width, height, ok := parseImageDimensions(size)
	return ok && width == height
}

func parseImageDimensions(size string) (int, int, bool) {
	canonical := strings.NewReplacer("*", "x", "X", "x", "×", "x").Replace(strings.TrimSpace(size))
	parts := strings.Split(canonical, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}
