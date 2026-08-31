package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ImageSelectionRequirements carries only the request fields needed before a
// channel is selected. It intentionally does not alter billing or relay data.
type ImageSelectionRequirements struct {
	CanonicalModel  string
	Size            string
	AspectRatio     string
	Tier            operation_setting.ImageResolutionTier
	Width           int
	Height          int
	ExactDimensions bool
}

func BuildImageSelectionRequirements(request *dto.ImageRequest) (*ImageSelectionRequirements, error) {
	if request == nil {
		return nil, errors.New("image request is nil")
	}
	requirements := &ImageSelectionRequirements{
		CanonicalModel: normalizeImageCapabilityModel(request.Model),
		Size:           strings.TrimSpace(request.Size),
	}
	if request.AspectRatio != nil {
		requirements.AspectRatio = strings.TrimSpace(*request.AspectRatio)
	}
	quote, configured, err := operation_setting.ResolveImageResolutionPrice(request.Model, request.Size)
	if err != nil {
		return nil, err
	}
	if configured {
		requirements.CanonicalModel = quote.PricingModel
		requirements.Tier = quote.Tier
		requirements.Size = quote.NormalizedSize
	} else {
		tier, normalizedSize, tierErr := imageSelectionTier(request.Size)
		if tierErr != nil {
			return nil, tierErr
		}
		requirements.Tier = tier
		requirements.Size = normalizedSize
	}
	if width, height, ok := parseImageDimensions(requirements.Size); ok {
		requirements.Width = width
		requirements.Height = height
		requirements.ExactDimensions = true
	}
	if requirements.AspectRatio != "" && !validImageAspectRatio(requirements.AspectRatio) {
		return nil, fmt.Errorf("invalid image aspect ratio %q", requirements.AspectRatio)
	}
	return requirements, nil
}

func ImageModelSelectionNames(modelName string, tier operation_setting.ImageResolutionTier) []string {
	canonical := normalizeImageCapabilityModel(modelName)
	names := []string{canonical}
	minRank := imageCapabilityTierRank(tier)
	for _, candidateTier := range []operation_setting.ImageResolutionTier{
		operation_setting.ImageResolutionTier1K,
		operation_setting.ImageResolutionTier2K,
		operation_setting.ImageResolutionTier4K,
	} {
		if minRank == 0 || imageCapabilityTierRank(candidateTier) >= minRank {
			names = append(names, canonical+"-"+string(candidateTier))
		}
	}
	return names
}

func imageSelectionTier(size string) (operation_setting.ImageResolutionTier, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(size))
	switch normalized {
	case "", "auto", "1k":
		return operation_setting.ImageResolutionTier1K, normalized, nil
	case "2k":
		return operation_setting.ImageResolutionTier2K, normalized, nil
	case "4k":
		return operation_setting.ImageResolutionTier4K, normalized, nil
	}
	width, height, ok := parseImageDimensions(normalized)
	if !ok {
		return "", normalized, fmt.Errorf("invalid image size %q", size)
	}
	switch {
	case width <= 1024 && height <= 1024:
		return operation_setting.ImageResolutionTier1K, fmt.Sprintf("%dx%d", width, height), nil
	case width <= 2048 && height <= 2048:
		return operation_setting.ImageResolutionTier2K, fmt.Sprintf("%dx%d", width, height), nil
	case width <= 4096 && height <= 4096:
		return operation_setting.ImageResolutionTier4K, fmt.Sprintf("%dx%d", width, height), nil
	default:
		return "", fmt.Sprintf("%dx%d", width, height), fmt.Errorf("image size %q exceeds the 4096x4096 limit", size)
	}
}

func validImageAspectRatio(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return false
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	return widthErr == nil && heightErr == nil && width > 0 && height > 0
}

func ValidateImageCapabilitySettings(settings dto.ChannelOtherSettings) error {
	support := strings.ToLower(strings.TrimSpace(settings.ImageDimensionSupport))
	switch support {
	case "", "auto", "any", "custom", "ratio", "aspect_ratio", "aspect-ratio", "square", "fixed_square", "fixed-square", "pending", "unknown":
	default:
		return fmt.Errorf("invalid image_dimension_support %q", settings.ImageDimensionSupport)
	}
	for modelName, capability := range settings.ImageModelCapabilities {
		if normalizeImageCapabilityModel(modelName) == "" {
			return errors.New("image_model_capabilities contains an empty model")
		}
		if !validImageCapabilityTier(capability.MaxTier) {
			return fmt.Errorf("image model %s has invalid max_tier %q", modelName, capability.MaxTier)
		}
		if capability.Shape != dto.ImageCapabilityShapeExact && capability.Shape != dto.ImageCapabilityShapeRatio {
			return fmt.Errorf("image model %s has invalid shape %q", modelName, capability.Shape)
		}
	}
	return nil
}

func ChannelImageCapabilityForModel(channel *Channel, modelName string) dto.ImageModelCapability {
	capability := dto.ImageModelCapability{MaxTier: string(operation_setting.ImageResolutionTier1K)}
	if channel == nil {
		return capability
	}
	settings := channel.GetOtherSettings()
	switch strings.ToLower(strings.TrimSpace(settings.ImageDimensionSupport)) {
	case "any", "custom":
		capability.MaxTier = string(operation_setting.ImageResolutionTier4K)
		capability.Shape = dto.ImageCapabilityShapeExact
	case "ratio", "aspect_ratio", "aspect-ratio":
		capability.MaxTier = string(operation_setting.ImageResolutionTier4K)
		capability.Shape = dto.ImageCapabilityShapeRatio
	}
	canonical := normalizeImageCapabilityModel(modelName)
	for configuredModel, modelCapability := range settings.ImageModelCapabilities {
		if normalizeImageCapabilityModel(configuredModel) == canonical {
			capability = modelCapability
			break
		}
	}
	return capability
}

func (requirements ImageSelectionRequirements) RequiresNonSquare() bool {
	if requirements.Width > 0 && requirements.Height > 0 {
		return requirements.Width != requirements.Height
	}
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

func ChannelSupportsImageRequest(channel *Channel, modelName string, requirements ImageSelectionRequirements) bool {
	if channel == nil {
		return false
	}
	capability := ChannelImageCapabilityForModel(channel, modelName)
	settings := channel.GetOtherSettings()
	support := strings.ToLower(strings.TrimSpace(settings.ImageDimensionSupport))
	modelCapabilityConfigured := false
	canonicalModel := normalizeImageCapabilityModel(modelName)
	for configuredModel := range settings.ImageModelCapabilities {
		if normalizeImageCapabilityModel(configuredModel) == canonicalModel {
			modelCapabilityConfigured = true
			break
		}
	}
	if requirements.Tier != "" && (modelCapabilityConfigured || support == "any" || support == "custom" || support == "ratio" || support == "aspect_ratio" || support == "aspect-ratio") && imageCapabilityTierRank(requirements.Tier) > imageCapabilityTierRank(operation_setting.ImageResolutionTier(capability.MaxTier)) {
		return false
	}
	if !requirements.RequiresNonSquare() {
		return true
	}
	if channelParamOverrideForcesSquare(channel, modelName) {
		return false
	}
	exactDimensions := requirements.ExactDimensions
	if _, _, ok := parseImageDimensions(requirements.Size); ok {
		exactDimensions = true
	}
	if exactDimensions && capability.Shape != dto.ImageCapabilityShapeExact {
		return false
	}
	return capability.Shape == dto.ImageCapabilityShapeExact || capability.Shape == dto.ImageCapabilityShapeRatio
}

func validImageCapabilityTier(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(operation_setting.ImageResolutionTier1K), string(operation_setting.ImageResolutionTier2K), string(operation_setting.ImageResolutionTier4K):
		return true
	default:
		return false
	}
}

func imageCapabilityTierRank(tier operation_setting.ImageResolutionTier) int {
	switch tier {
	case operation_setting.ImageResolutionTier1K:
		return 1
	case operation_setting.ImageResolutionTier2K:
		return 2
	case operation_setting.ImageResolutionTier4K:
		return 4
	default:
		return 0
	}
}

func normalizeImageCapabilityModel(modelName string) string {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if idx := strings.LastIndex(modelName, "/"); idx >= 0 {
		modelName = modelName[idx+1:]
	}
	for _, suffix := range []string{"-1k", "-2k", "-4k"} {
		modelName = strings.TrimSuffix(modelName, suffix)
	}
	return modelName
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
