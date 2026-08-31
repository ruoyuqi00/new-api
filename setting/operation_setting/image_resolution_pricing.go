package operation_setting

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type ImageResolutionTier string

const (
	ImageResolutionTier1K ImageResolutionTier = "1k"
	ImageResolutionTier2K ImageResolutionTier = "2k"
	ImageResolutionTier4K ImageResolutionTier = "4k"
)

type ImageResolutionPricePolicy struct {
	Prices      map[ImageResolutionTier]float64 `json:"prices"`
	DefaultTier ImageResolutionTier             `json:"default_tier"`
}

type ImageResolutionPriceQuote struct {
	RequestedModel   string
	PricingModel     string
	RequestedSize    string
	NormalizedSize   string
	Tier             ImageResolutionTier
	AliasMinimumTier ImageResolutionTier
	UnitPrice        float64
}

type ImageResolutionPricingMetadata struct {
	PricingModel     string                          `json:"pricing_model"`
	DefaultTier      ImageResolutionTier             `json:"default_tier"`
	Prices           map[ImageResolutionTier]float64 `json:"prices"`
	AliasMinimumTier ImageResolutionTier             `json:"alias_minimum_tier,omitempty"`
}

type ImageResolutionPriceSetting struct {
	Models map[string]ImageResolutionPricePolicy `json:"models"`
}

type imageResolutionPriceIndex struct {
	models map[string]ImageResolutionPricePolicy
}

var imageResolutionPriceSetting = ImageResolutionPriceSetting{
	Models: map[string]ImageResolutionPricePolicy{
		"gpt-image-2": {
			Prices: map[ImageResolutionTier]float64{
				ImageResolutionTier1K: 0.01,
				ImageResolutionTier2K: 0.04,
				ImageResolutionTier4K: 0.045,
			},
			DefaultTier: ImageResolutionTier1K,
		},
		"nano-banana-pro": {
			Prices: map[ImageResolutionTier]float64{
				ImageResolutionTier1K: 0.086666666667,
				ImageResolutionTier2K: 0.108333333333,
				ImageResolutionTier4K: 0.161416666667,
			},
			DefaultTier: ImageResolutionTier1K,
		},
		"nano-banana2": {
			Prices: map[ImageResolutionTier]float64{
				ImageResolutionTier1K: 0.063916666667,
				ImageResolutionTier2K: 0.086666666667,
				ImageResolutionTier4K: 0.13,
			},
			DefaultTier: ImageResolutionTier1K,
		},
	},
}

var imageResolutionPriceIndexValue atomic.Pointer[imageResolutionPriceIndex]

func init() {
	config.GlobalConfig.Register("image_resolution_price_setting", &imageResolutionPriceSetting)
	RebuildImageResolutionPriceIndex()
}

func normalizeImageResolutionPricingModel(modelName string) string {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if idx := strings.LastIndex(modelName, "/"); idx >= 0 {
		modelName = modelName[idx+1:]
	}
	return modelName
}

func validateImageResolutionPricePolicies(models map[string]ImageResolutionPricePolicy) error {
	if len(models) == 0 {
		return fmt.Errorf("image resolution price policies cannot be empty")
	}

	normalizedNames := make(map[string]struct{}, len(models))
	for modelName, policy := range models {
		normalizedName := normalizeImageResolutionPricingModel(modelName)
		if normalizedName == "" {
			return fmt.Errorf("image resolution pricing model cannot be empty")
		}
		if _, exists := normalizedNames[normalizedName]; exists {
			return fmt.Errorf("duplicate image resolution pricing model %s", normalizedName)
		}
		normalizedNames[normalizedName] = struct{}{}

		if len(policy.Prices) != 3 {
			return fmt.Errorf("model %s must configure exactly 1k, 2k, and 4k prices", normalizedName)
		}
		for _, tier := range []ImageResolutionTier{ImageResolutionTier1K, ImageResolutionTier2K, ImageResolutionTier4K} {
			price, ok := policy.Prices[tier]
			if !ok {
				return fmt.Errorf("model %s is missing %s price", normalizedName, tier)
			}
			if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
				return fmt.Errorf("model %s has invalid %s price", normalizedName, tier)
			}
		}
		if _, ok := policy.Prices[policy.DefaultTier]; !ok {
			return fmt.Errorf("model %s has invalid default tier %s", normalizedName, policy.DefaultTier)
		}
		if policy.Prices[ImageResolutionTier1K] > policy.Prices[ImageResolutionTier2K] ||
			policy.Prices[ImageResolutionTier2K] > policy.Prices[ImageResolutionTier4K] {
			return fmt.Errorf("model %s prices must be non-decreasing from 1k to 4k", normalizedName)
		}
	}
	return nil
}

func buildImageResolutionPriceIndex(models map[string]ImageResolutionPricePolicy) (*imageResolutionPriceIndex, error) {
	if err := validateImageResolutionPricePolicies(models); err != nil {
		return nil, err
	}

	index := &imageResolutionPriceIndex{models: make(map[string]ImageResolutionPricePolicy, len(models))}
	for modelName, policy := range models {
		prices := make(map[ImageResolutionTier]float64, len(policy.Prices))
		for tier, price := range policy.Prices {
			prices[tier] = price
		}
		index.models[normalizeImageResolutionPricingModel(modelName)] = ImageResolutionPricePolicy{
			Prices:      prices,
			DefaultTier: policy.DefaultTier,
		}
	}
	return index, nil
}

func RebuildImageResolutionPriceIndex() {
	index, err := buildImageResolutionPriceIndex(imageResolutionPriceSetting.Models)
	if err != nil {
		return
	}
	imageResolutionPriceIndexValue.Store(index)
}

func ValidateImageResolutionPriceJSONString(value string) error {
	var models map[string]ImageResolutionPricePolicy
	if err := common.UnmarshalJsonStr(value, &models); err != nil {
		return fmt.Errorf("invalid image resolution price JSON: %w", err)
	}
	return validateImageResolutionPricePolicies(models)
}

func ImageResolutionPriceSetting2JSONString() string {
	index := imageResolutionPriceIndexValue.Load()
	if index == nil {
		return "{}"
	}
	data, err := common.Marshal(index.models)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func resolveImageResolutionPricingModel(index *imageResolutionPriceIndex, modelName string) (string, ImageResolutionTier, bool) {
	modelName = normalizeImageResolutionPricingModel(modelName)
	if _, ok := index.models[modelName]; ok {
		return modelName, "", true
	}
	for _, tier := range []ImageResolutionTier{ImageResolutionTier4K, ImageResolutionTier2K, ImageResolutionTier1K} {
		suffix := "-" + string(tier)
		if !strings.HasSuffix(modelName, suffix) {
			continue
		}
		pricingModel := strings.TrimSuffix(modelName, suffix)
		if _, ok := index.models[pricingModel]; ok {
			return pricingModel, tier, true
		}
	}
	return "", "", false
}

func tierRank(tier ImageResolutionTier) int {
	switch tier {
	case ImageResolutionTier1K:
		return 1
	case ImageResolutionTier2K:
		return 2
	case ImageResolutionTier4K:
		return 4
	default:
		return 0
	}
}

func parseImageResolutionTier(size string, defaultTier ImageResolutionTier) (ImageResolutionTier, string, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(size)), ""))
	if normalized == "" || normalized == "auto" {
		return defaultTier, "auto", nil
	}
	if normalized == string(ImageResolutionTier1K) || normalized == string(ImageResolutionTier2K) || normalized == string(ImageResolutionTier4K) {
		return ImageResolutionTier(normalized), normalized, nil
	}

	normalized = strings.ReplaceAll(normalized, "*", "x")
	normalized = strings.ReplaceAll(normalized, "\u00d7", "x")
	if strings.Count(normalized, "x") != 1 {
		return "", normalized, fmt.Errorf("invalid image size %q", size)
	}
	parts := strings.SplitN(normalized, "x", 2)
	width, widthErr := strconv.Atoi(parts[0])
	height, heightErr := strconv.Atoi(parts[1])
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return "", normalized, fmt.Errorf("invalid image size %q", size)
	}

	normalized = fmt.Sprintf("%dx%d", width, height)
	switch {
	case width <= 1024 && height <= 1024:
		return ImageResolutionTier1K, normalized, nil
	case width <= 2048 && height <= 2048:
		return ImageResolutionTier2K, normalized, nil
	case width <= 4096 && height <= 4096:
		return ImageResolutionTier4K, normalized, nil
	default:
		return "", normalized, fmt.Errorf("image size %q exceeds the 4096x4096 limit", size)
	}
}

func ResolveImageResolutionPrice(modelName, size string) (ImageResolutionPriceQuote, bool, error) {
	index := imageResolutionPriceIndexValue.Load()
	if index == nil {
		return ImageResolutionPriceQuote{}, false, nil
	}
	pricingModel, aliasMinimumTier, configured := resolveImageResolutionPricingModel(index, modelName)
	if !configured {
		return ImageResolutionPriceQuote{}, false, nil
	}

	policy := index.models[pricingModel]
	tier, normalizedSize, err := parseImageResolutionTier(size, policy.DefaultTier)
	if err != nil {
		return ImageResolutionPriceQuote{}, true, err
	}
	if tierRank(aliasMinimumTier) > tierRank(tier) {
		tier = aliasMinimumTier
	}
	unitPrice, ok := policy.Prices[tier]
	if !ok {
		return ImageResolutionPriceQuote{}, true, fmt.Errorf("model %s has no price for image tier %s", pricingModel, tier)
	}

	return ImageResolutionPriceQuote{
		RequestedModel:   modelName,
		PricingModel:     pricingModel,
		RequestedSize:    size,
		NormalizedSize:   normalizedSize,
		Tier:             tier,
		AliasMinimumTier: aliasMinimumTier,
		UnitPrice:        unitPrice,
	}, true, nil
}

func GetImageResolutionPricingMetadata(modelName string) (ImageResolutionPricingMetadata, bool) {
	index := imageResolutionPriceIndexValue.Load()
	if index == nil {
		return ImageResolutionPricingMetadata{}, false
	}
	pricingModel, aliasMinimumTier, configured := resolveImageResolutionPricingModel(index, modelName)
	if !configured {
		return ImageResolutionPricingMetadata{}, false
	}
	policy := index.models[pricingModel]
	prices := make(map[ImageResolutionTier]float64, len(policy.Prices))
	for tier, price := range policy.Prices {
		prices[tier] = price
	}
	return ImageResolutionPricingMetadata{
		PricingModel:     pricingModel,
		DefaultTier:      policy.DefaultTier,
		Prices:           prices,
		AliasMinimumTier: aliasMinimumTier,
	}, true
}
