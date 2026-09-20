package operation_setting

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type VideoBillingUnit string

const (
	VideoBillingUnitPerSecond         VideoBillingUnit = "per_second"
	VideoBillingUnitPerSuccessfulTask VideoBillingUnit = "per_successful_task"
	VideoBillingUnitPerMillionTokens  VideoBillingUnit = "per_1m_video_tokens"
)

type VideoTierPricePoint struct {
	Standard           float64  `json:"standard"`
	WithReferenceVideo *float64 `json:"with_reference_video,omitempty"`
}

type VideoTierPriceQuote struct {
	Model             string
	Tier              string
	BillingUnit       VideoBillingUnit
	UnitPrice         float64
	Inherited         bool
	HasReferenceVideo bool
}

type VideoTierPricingMetadata struct {
	BillingUnit VideoBillingUnit               `json:"billing_unit"`
	Inherited   bool                           `json:"inherited"`
	Tiers       []string                       `json:"tiers"`
	Prices      map[string]VideoTierPricePoint `json:"prices"`
}

type VideoTierPriceSetting struct {
	Models map[string]map[string]VideoTierPricePoint `json:"models"`
}

type videoTierPriceDefinition struct {
	PublicModelName               string
	BillingUnit                   VideoBillingUnit
	BaseTier                      string
	DefaultTier                   string
	Tiers                         []string
	LegacyPrices                  map[string]VideoTierPricePoint
	Aliases                       map[string]string
	RequiresReferenceVideoPricing bool
}

type videoTierPriceIndex struct {
	models           map[string]map[string]VideoTierPricePoint
	serializedModels map[string]map[string]VideoTierPricePoint
}

func videoPricePoint(standard float64) VideoTierPricePoint {
	return VideoTierPricePoint{Standard: standard}
}

func videoReferencePricePoint(standard, withReferenceVideo float64) VideoTierPricePoint {
	return VideoTierPricePoint{Standard: standard, WithReferenceVideo: &withReferenceVideo}
}

func videoTierAliases(tiers map[string][]string) map[string]string {
	aliases := make(map[string]string)
	for tier, values := range tiers {
		aliases[strings.ToLower(tier)] = strings.ToLower(tier)
		for _, value := range values {
			aliases[strings.ToLower(value)] = strings.ToLower(tier)
		}
	}
	return aliases
}

var videoTierPriceDefinitions = map[string]videoTierPriceDefinition{
	"seedance-2-0-mini-official": {
		PublicModelName: "seedance-2-0-mini-official",
		BillingUnit:     VideoBillingUnitPerMillionTokens, BaseTier: "720p", DefaultTier: "720p",
		Tiers: []string{"480p", "720p"}, RequiresReferenceVideoPricing: true,
		LegacyPrices: map[string]VideoTierPricePoint{
			"480p": videoReferencePricePoint(11.5, 7), "720p": videoReferencePricePoint(11.5, 7),
		},
		Aliases: videoTierAliases(map[string][]string{"480p": nil, "720p": nil}),
	},
	"seedance-2-0-fast-official": {
		PublicModelName: "seedance-2-0-fast-official",
		BillingUnit:     VideoBillingUnitPerMillionTokens, BaseTier: "720p", DefaultTier: "720p",
		Tiers: []string{"480p", "720p"}, RequiresReferenceVideoPricing: true,
		LegacyPrices: map[string]VideoTierPricePoint{
			"480p": videoReferencePricePoint(24.05, 14.3), "720p": videoReferencePricePoint(24.05, 14.3),
		},
		Aliases: videoTierAliases(map[string][]string{"480p": nil, "720p": nil}),
	},
	"seedance-2-0-official": {
		PublicModelName: "seedance-2-0-official",
		BillingUnit:     VideoBillingUnitPerMillionTokens, BaseTier: "720p", DefaultTier: "720p",
		Tiers: []string{"480p", "720p", "1080p", "4k"}, RequiresReferenceVideoPricing: true,
		LegacyPrices: map[string]VideoTierPricePoint{
			"480p": videoReferencePricePoint(36.8, 22.4), "720p": videoReferencePricePoint(36.8, 22.4),
			"1080p": videoReferencePricePoint(40.8, 24.8), "4k": videoReferencePricePoint(20.8, 12.8),
		},
		Aliases: videoTierAliases(map[string][]string{"480p": nil, "720p": nil, "1080p": nil, "4k": nil}),
	},
	"seedance-2-5-official": {
		PublicModelName: "seedance-2-5-official",
		BillingUnit:     VideoBillingUnitPerMillionTokens, BaseTier: "720p", DefaultTier: "720p",
		Tiers: []string{"720p", "1080p"}, RequiresReferenceVideoPricing: true,
		LegacyPrices: map[string]VideoTierPricePoint{
			"720p": videoReferencePricePoint(59.5, 35.7), "1080p": videoReferencePricePoint(65.45, 39.1),
		},
		Aliases: videoTierAliases(map[string][]string{"720p": nil, "1080p": nil}),
	},
	"minimax-h3": {
		PublicModelName: "minimax-h3",
		BillingUnit:     VideoBillingUnitPerSecond, BaseTier: "480p", DefaultTier: "480p",
		Tiers: []string{"480p", "768p", "1080p", "2k", "4k"},
		LegacyPrices: map[string]VideoTierPricePoint{
			"480p": videoPricePoint(0.10), "768p": videoPricePoint(0.16), "1080p": videoPricePoint(0.18),
			"2k": videoPricePoint(0.26), "4k": videoPricePoint(0.36),
		},
		Aliases: videoTierAliases(map[string][]string{
			"480p":  {"864x480", "480x864", "640x640", "544x800", "800x544", "576x736", "736x576", "992x416"},
			"768p":  {"1376x768", "768x1376", "1024x1024", "832x1248", "1248x832", "896x1184", "1184x896", "1568x672"},
			"1080p": {"1920x1088", "1088x1920", "1440x1440", "1184x1760", "1760x1184", "1248x1664", "1664x1248", "2208x960"},
			"2k":    nil, "4k": nil,
		}),
	},
	"wan3.0-video": {
		PublicModelName: "wan3.0-video",
		BillingUnit:     VideoBillingUnitPerSecond, BaseTier: "480p", DefaultTier: "720p", Tiers: []string{"480p", "720p", "1080p"},
		LegacyPrices: map[string]VideoTierPricePoint{
			"480p": videoPricePoint(0.27), "720p": videoPricePoint(0.36), "1080p": videoPricePoint(0.72),
		},
		Aliases: videoTierAliases(map[string][]string{
			"480p": {"854x480", "832x480", "864x480", "480x854"},
			"720p": {"1280x720", "720x1280"}, "1080p": {"1920x1080", "1080x1920"},
		}),
	},
	"wan3.0-video-prime": {
		PublicModelName: "wan3.0-video-prime",
		BillingUnit:     VideoBillingUnitPerSecond, BaseTier: "480p", DefaultTier: "720p", Tiers: []string{"480p", "720p", "1080p"},
		LegacyPrices: map[string]VideoTierPricePoint{
			"480p": videoPricePoint(0.40), "720p": videoPricePoint(0.54), "1080p": videoPricePoint(1.08),
		},
		Aliases: videoTierAliases(map[string][]string{
			"480p": {"854x480", "832x480", "864x480", "480x854"},
			"720p": {"1280x720", "720x1280"}, "1080p": {"1920x1080", "1080x1920"},
		}),
	},
	"seedance2.0-9-3-3-pt": {
		PublicModelName: "seedance2.0-9-3-3-PT",
		BillingUnit:     VideoBillingUnitPerSecond, BaseTier: "480p", DefaultTier: "720p", Tiers: []string{"480p", "720p"},
		LegacyPrices: map[string]VideoTierPricePoint{"480p": videoPricePoint(0.34), "720p": videoPricePoint(0.42)},
		Aliases:      videoTierAliases(map[string][]string{"480p": {"854x480", "832x480", "864x480", "480x854"}, "720p": {"1280x720", "720x1280"}}),
	},
	"seedance2.5-30-10-10-pt": {
		PublicModelName: "seedance2.5-30-10-10-PT",
		BillingUnit:     VideoBillingUnitPerSecond, BaseTier: "480p", DefaultTier: "720p", Tiers: []string{"480p", "720p"},
		LegacyPrices: map[string]VideoTierPricePoint{"480p": videoPricePoint(0.45), "720p": videoPricePoint(0.67)},
		Aliases:      videoTierAliases(map[string][]string{"480p": {"854x480", "832x480", "864x480", "480x854"}, "720p": {"1280x720", "720x1280"}}),
	},
	"seedance2.0-fast-pt": {
		PublicModelName: "seedance2.0-fast-PT",
		BillingUnit:     VideoBillingUnitPerSecond, BaseTier: "480p", DefaultTier: "720p", Tiers: []string{"480p", "720p"},
		LegacyPrices: map[string]VideoTierPricePoint{"480p": videoPricePoint(0.30), "720p": videoPricePoint(0.36)},
		Aliases:      videoTierAliases(map[string][]string{"480p": {"854x480", "832x480", "864x480", "480x854"}, "720p": {"1280x720", "720x1280"}}),
	},
	"grok-v1.5-video": {
		PublicModelName: "grok-v1.5-video",
		BillingUnit:     VideoBillingUnitPerSuccessfulTask, BaseTier: "720p", DefaultTier: "720p", Tiers: []string{"720p", "1080p"},
		LegacyPrices: map[string]VideoTierPricePoint{"720p": videoPricePoint(0.60), "1080p": videoPricePoint(0.60)},
		Aliases:      videoTierAliases(map[string][]string{"720p": {"1280x720", "720x1280"}, "1080p": {"1920x1080", "1080x1920"}}),
	},
}

var videoTierPriceSetting = VideoTierPriceSetting{Models: map[string]map[string]VideoTierPricePoint{}}
var videoTierPriceIndexValue atomic.Pointer[videoTierPriceIndex]

func init() {
	config.GlobalConfig.Register("video_pricing_setting", &videoTierPriceSetting)
	RebuildVideoTierPriceIndex()
}

func normalizeVideoPricingModel(modelName string) string {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if index := strings.LastIndex(modelName, "/"); index >= 0 {
		modelName = modelName[index+1:]
	}
	return modelName
}

func validVideoTierPrice(price float64) bool {
	return price > 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

func validateVideoTierPriceModels(models map[string]map[string]VideoTierPricePoint) error {
	normalizedModels := make(map[string]struct{}, len(models))
	for modelName, prices := range models {
		normalizedModel := normalizeVideoPricingModel(modelName)
		definition, ok := videoTierPriceDefinitions[normalizedModel]
		if !ok {
			return fmt.Errorf("unknown video pricing model %s", modelName)
		}
		if _, duplicate := normalizedModels[normalizedModel]; duplicate {
			return fmt.Errorf("duplicate video pricing model %s", normalizedModel)
		}
		normalizedModels[normalizedModel] = struct{}{}
		if len(prices) != len(definition.Tiers) {
			return fmt.Errorf("model %s must configure all supported video tiers", normalizedModel)
		}

		normalizedTiers := make(map[string]struct{}, len(prices))
		for tierName, price := range prices {
			normalizedTier := strings.ToLower(strings.TrimSpace(tierName))
			if _, ok := definition.LegacyPrices[normalizedTier]; !ok {
				return fmt.Errorf("model %s has unsupported video tier %s", normalizedModel, tierName)
			}
			if _, duplicate := normalizedTiers[normalizedTier]; duplicate {
				return fmt.Errorf("model %s has duplicate video tier %s", normalizedModel, normalizedTier)
			}
			normalizedTiers[normalizedTier] = struct{}{}
			if !validVideoTierPrice(price.Standard) {
				return fmt.Errorf("model %s has invalid %s standard price", normalizedModel, normalizedTier)
			}
			if definition.RequiresReferenceVideoPricing {
				if price.WithReferenceVideo == nil || !validVideoTierPrice(*price.WithReferenceVideo) {
					return fmt.Errorf("model %s has invalid %s reference-video price", normalizedModel, normalizedTier)
				}
			} else if price.WithReferenceVideo != nil {
				return fmt.Errorf("model %s does not support a separate reference-video price", normalizedModel)
			}
		}
		for _, tier := range definition.Tiers {
			if _, ok := normalizedTiers[tier]; !ok {
				return fmt.Errorf("model %s is missing video tier %s", normalizedModel, tier)
			}
		}
	}
	return nil
}

func copyVideoTierPricePoint(point VideoTierPricePoint) VideoTierPricePoint {
	copy := VideoTierPricePoint{Standard: point.Standard}
	if point.WithReferenceVideo != nil {
		value := *point.WithReferenceVideo
		copy.WithReferenceVideo = &value
	}
	return copy
}

func buildVideoTierPriceIndex(models map[string]map[string]VideoTierPricePoint) (*videoTierPriceIndex, error) {
	if err := validateVideoTierPriceModels(models); err != nil {
		return nil, err
	}
	index := &videoTierPriceIndex{
		models:           make(map[string]map[string]VideoTierPricePoint, len(models)),
		serializedModels: make(map[string]map[string]VideoTierPricePoint, len(models)),
	}
	for modelName, prices := range models {
		normalizedModel := normalizeVideoPricingModel(modelName)
		definition := videoTierPriceDefinitions[normalizedModel]
		normalizedPrices := make(map[string]VideoTierPricePoint, len(prices))
		for tier, point := range prices {
			normalizedPrices[strings.ToLower(strings.TrimSpace(tier))] = copyVideoTierPricePoint(point)
		}
		index.models[normalizedModel] = normalizedPrices
		index.serializedModels[definition.PublicModelName] = normalizedPrices
	}
	return index, nil
}

func RebuildVideoTierPriceIndex() {
	index, err := buildVideoTierPriceIndex(videoTierPriceSetting.Models)
	if err != nil {
		return
	}
	videoTierPriceIndexValue.Store(index)
}

func ValidateVideoTierPriceJSONString(value string) error {
	var models map[string]map[string]VideoTierPricePoint
	if err := common.UnmarshalJsonStr(value, &models); err != nil {
		return fmt.Errorf("invalid video tier price JSON: %w", err)
	}
	return validateVideoTierPriceModels(models)
}

func VideoTierPriceSetting2JSONString() string {
	index := videoTierPriceIndexValue.Load()
	if index == nil {
		return "{}"
	}
	data, err := common.Marshal(index.serializedModels)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func resolveVideoTier(definition videoTierPriceDefinition, resolution string) (string, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(resolution)), ""))
	if normalized == "" || normalized == "auto" {
		return definition.DefaultTier, nil
	}
	tier, ok := definition.Aliases[normalized]
	if !ok {
		return "", fmt.Errorf("unsupported video pricing tier %q", resolution)
	}
	return tier, nil
}

func effectiveVideoTierPrices(modelName string, definition videoTierPriceDefinition, inheritedBasePrice float64) (map[string]VideoTierPricePoint, bool, error) {
	index := videoTierPriceIndexValue.Load()
	if index != nil {
		if override, ok := index.models[modelName]; ok {
			prices := make(map[string]VideoTierPricePoint, len(override))
			for tier, point := range override {
				prices[tier] = copyVideoTierPricePoint(point)
			}
			return prices, false, nil
		}
	}
	if !validVideoTierPrice(inheritedBasePrice) {
		return nil, true, fmt.Errorf("model %s has invalid inherited video base price", modelName)
	}
	basePoint := definition.LegacyPrices[definition.BaseTier]
	if !validVideoTierPrice(basePoint.Standard) {
		return nil, true, fmt.Errorf("model %s has invalid legacy video base price", modelName)
	}
	prices := make(map[string]VideoTierPricePoint, len(definition.LegacyPrices))
	for tier, legacy := range definition.LegacyPrices {
		point := VideoTierPricePoint{Standard: inheritedBasePrice * legacy.Standard / basePoint.Standard}
		if definition.RequiresReferenceVideoPricing && legacy.WithReferenceVideo != nil {
			value := inheritedBasePrice * *legacy.WithReferenceVideo / basePoint.Standard
			point.WithReferenceVideo = &value
		}
		prices[tier] = point
	}
	return prices, true, nil
}

func ResolveVideoTierPrice(modelName, resolution string, hasReferenceVideo bool, inheritedBasePrice float64) (VideoTierPriceQuote, bool, error) {
	normalizedModel := normalizeVideoPricingModel(modelName)
	definition, configured := videoTierPriceDefinitions[normalizedModel]
	if !configured {
		return VideoTierPriceQuote{}, false, nil
	}
	tier, err := resolveVideoTier(definition, resolution)
	if err != nil {
		return VideoTierPriceQuote{}, true, err
	}
	prices, inherited, err := effectiveVideoTierPrices(normalizedModel, definition, inheritedBasePrice)
	if err != nil {
		return VideoTierPriceQuote{}, true, err
	}
	point, ok := prices[tier]
	if !ok {
		return VideoTierPriceQuote{}, true, fmt.Errorf("model %s has no price for video tier %s", normalizedModel, tier)
	}
	unitPrice := point.Standard
	if hasReferenceVideo && definition.RequiresReferenceVideoPricing {
		if point.WithReferenceVideo == nil {
			return VideoTierPriceQuote{}, true, fmt.Errorf("model %s has no reference-video price for tier %s", normalizedModel, tier)
		}
		unitPrice = *point.WithReferenceVideo
	}
	return VideoTierPriceQuote{
		Model: normalizedModel, Tier: tier, BillingUnit: definition.BillingUnit,
		UnitPrice: unitPrice, Inherited: inherited, HasReferenceVideo: hasReferenceVideo,
	}, true, nil
}

func GetVideoTierPricingMetadata(modelName string, inheritedBasePrice float64) (VideoTierPricingMetadata, bool) {
	normalizedModel := normalizeVideoPricingModel(modelName)
	definition, configured := videoTierPriceDefinitions[normalizedModel]
	if !configured {
		return VideoTierPricingMetadata{}, false
	}
	prices, inherited, err := effectiveVideoTierPrices(normalizedModel, definition, inheritedBasePrice)
	if err != nil {
		return VideoTierPricingMetadata{}, false
	}
	return VideoTierPricingMetadata{
		BillingUnit: definition.BillingUnit,
		Inherited:   inherited,
		Tiers:       append([]string(nil), definition.Tiers...),
		Prices:      prices,
	}, true
}
