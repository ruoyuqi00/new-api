package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	YucoreMediaKindImage = "image"
	YucoreMediaKindVideo = "video"
)

type YucoreMediaCatalog struct {
	DefaultGroup string                    `json:"default_group"`
	Groups       []YucoreMediaCatalogGroup `json:"groups"`
}

type YucoreMediaCatalogGroup struct {
	Id          string                    `json:"id"`
	Description string                    `json:"description"`
	Ratio       float64                   `json:"ratio"`
	Models      []YucoreMediaCatalogModel `json:"models"`
}

type YucoreMediaCatalogPricing struct {
	Unit     string  `json:"unit,omitempty"`
	Amount   float64 `json:"amount,omitempty"`
	Currency string  `json:"currency,omitempty"`
	Display  string  `json:"display,omitempty"`
}

type YucoreMediaCatalogInputLimits struct {
	MaxPromptChars     int `json:"max_prompt_chars,omitempty"`
	MaxReferenceImages int `json:"max_reference_images,omitempty"`
	MaxFileSizeMB      int `json:"max_file_size_mb,omitempty"`
}

type YucoreMediaCatalogModel struct {
	Id           string                        `json:"id"`
	Name         string                        `json:"name"`
	Description  string                        `json:"description,omitempty"`
	Kind         string                        `json:"kind"`
	Modes        []string                      `json:"modes"`
	Sizes        []string                      `json:"sizes,omitempty"`
	AspectRatios []string                      `json:"aspect_ratios,omitempty"`
	Qualities    []string                      `json:"qualities,omitempty"`
	Formats      []string                      `json:"formats,omitempty"`
	Counts       []int                         `json:"counts,omitempty"`
	Durations    []int                         `json:"durations,omitempty"`
	InputLimits  YucoreMediaCatalogInputLimits `json:"input_limits"`
	Pricing      YucoreMediaCatalogPricing     `json:"pricing"`
	Async        bool                          `json:"async"`
}

func yucoreMediaCapabilityForModel(capabilities map[string]model.YucoreMediaModelCapability, modelID string) (model.YucoreMediaModelCapability, bool) {
	if capability, ok := capabilities[modelID]; ok {
		return capability, true
	}
	for configuredModel, capability := range capabilities {
		if strings.EqualFold(strings.TrimSpace(configuredModel), modelID) {
			return capability, true
		}
	}
	return model.YucoreMediaModelCapability{}, false
}

func yucoreMediaCapabilityAllows(capability model.YucoreMediaModelCapability, parameter string) bool {
	for _, allowed := range capability.AllowedParameters {
		if strings.EqualFold(strings.TrimSpace(allowed), parameter) {
			return true
		}
	}
	return false
}

func yucoreMediaKindsForAbility(ability model.AbilityWithChannel, capability model.YucoreMediaModelCapability, configured bool) []string {
	kinds := make([]string, 0, 2)
	for _, endpointType := range common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model) {
		switch endpointType {
		case constant.EndpointTypeImageGeneration:
			kinds = append(kinds, YucoreMediaKindImage)
		case constant.EndpointTypeOpenAIVideo:
			kinds = append(kinds, YucoreMediaKindVideo)
		}
	}

	transport := strings.ToLower(strings.TrimSpace(capability.Transport))
	if configured && (transport == "sync-image" || strings.Contains(strings.ToLower(capability.CreatePath), "/images")) {
		kinds = append(kinds, YucoreMediaKindImage)
	}
	if configured && (strings.Contains(transport, "video") || strings.Contains(strings.ToLower(capability.CreatePath), "/videos")) {
		kinds = append(kinds, YucoreMediaKindVideo)
	}

	lowerModelID := strings.ToLower(strings.TrimSpace(ability.Model))
	if len(kinds) == 0 && (strings.Contains(lowerModelID, "image") || strings.Contains(lowerModelID, "imagen") || strings.Contains(lowerModelID, "banana") || strings.Contains(lowerModelID, "firefly")) {
		kinds = append(kinds, YucoreMediaKindImage)
	}
	if len(kinds) == 0 && (strings.Contains(lowerModelID, "video") || strings.HasPrefix(lowerModelID, "veo-") || strings.Contains(lowerModelID, "seedance") || strings.HasPrefix(lowerModelID, "sora-")) {
		kinds = append(kinds, YucoreMediaKindVideo)
	}

	seen := make(map[string]struct{}, len(kinds))
	unique := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		unique = append(unique, kind)
	}
	return unique
}

func buildYucoreMediaCatalogModel(modelID string, kind string, capability model.YucoreMediaModelCapability, configured bool, groupRatio float64) YucoreMediaCatalogModel {
	item := YucoreMediaCatalogModel{
		Id:   modelID,
		Name: modelID,
		Kind: kind,
		InputLimits: YucoreMediaCatalogInputLimits{
			MaxPromptChars: 4000,
			MaxFileSizeMB:  25,
		},
	}
	if kind == YucoreMediaKindVideo {
		item.Modes = []string{"text-to-video"}
		item.Async = true
		item.InputLimits.MaxPromptChars = 2000
		if configured && (yucoreMediaCapabilityAllows(capability, "image") || yucoreMediaCapabilityAllows(capability, "image_url") || yucoreMediaCapabilityAllows(capability, "image_urls") || yucoreMediaCapabilityAllows(capability, "images")) {
			item.Modes = append(item.Modes, "image-to-video")
			item.InputLimits.MaxReferenceImages = capability.MaxReferenceImages
			if item.InputLimits.MaxReferenceImages <= 0 {
				item.InputLimits.MaxReferenceImages = 1
			}
		}
		if configured && capability.FixedDurationSeconds > 0 {
			item.Durations = []int{capability.FixedDurationSeconds}
		}
	} else {
		item.Modes = []string{"text-to-image"}
		item.Counts = []int{1}
		if configured && strings.TrimSpace(capability.EditPath) != "" {
			item.Modes = append(item.Modes, "image-to-image")
			item.InputLimits.MaxReferenceImages = capability.MaxReferenceImages
			if item.InputLimits.MaxReferenceImages <= 0 {
				item.InputLimits.MaxReferenceImages = 16
			}
		}
	}

	if price, ok := ratio_setting.GetModelPrice(modelID, false); ok {
		price *= groupRatio
		unit := "per_call"
		if kind == YucoreMediaKindVideo && !model.YucoreMediaModelUsesPerCallPricing(modelID) {
			unit = "per_second"
		}
		item.Pricing = YucoreMediaCatalogPricing{
			Unit:     unit,
			Amount:   price,
			Currency: "CNY",
			Display:  fmt.Sprintf("CNY %g", price),
		}
	}
	return item
}

func yucoreMediaCatalogGroupOrder(usableGroups map[string]string, preferredGroup string) []string {
	ordered := make([]string, 0, len(usableGroups))
	seen := make(map[string]struct{}, len(usableGroups))
	appendGroup := func(group string) {
		group = strings.TrimSpace(group)
		if group == "" {
			return
		}
		if _, usable := usableGroups[group]; !usable {
			return
		}
		if _, exists := seen[group]; exists {
			return
		}
		seen[group] = struct{}{}
		ordered = append(ordered, group)
	}
	appendGroup(preferredGroup)
	appendGroup("auto")
	rest := make([]string, 0, len(usableGroups))
	for group := range usableGroups {
		if _, exists := seen[group]; !exists {
			rest = append(rest, group)
		}
	}
	sort.Strings(rest)
	for _, group := range rest {
		appendGroup(group)
	}
	return ordered
}

func BuildYucoreMediaCatalog(userID int) (*YucoreMediaCatalog, error) {
	user, err := model.GetUserCache(userID)
	if err != nil {
		return nil, err
	}
	usableGroups := GetUserUsableGroups(user.Group)
	preferredGroup, capabilities := model.GetYucoreMediaCatalogSettings()
	catalog := &YucoreMediaCatalog{Groups: make([]YucoreMediaCatalogGroup, 0)}

	for _, groupID := range yucoreMediaCatalogGroupOrder(usableGroups, preferredGroup) {
		routeGroups := []string{groupID}
		if groupID == "auto" {
			routeGroups = GetUserAutoGroup(user.Group)
		}
		abilities, err := model.GetActiveAbilitiesForGroups(routeGroups)
		if err != nil {
			return nil, err
		}
		groupRatio := GetUserGroupRatio(user.Group, groupID)
		models := make([]YucoreMediaCatalogModel, 0)
		seenModels := make(map[string]struct{})
		for _, ability := range abilities {
			capability, configured := yucoreMediaCapabilityForModel(capabilities, ability.Model)
			for _, kind := range yucoreMediaKindsForAbility(ability, capability, configured) {
				key := strings.ToLower(ability.Model) + "\x00" + kind
				if _, exists := seenModels[key]; exists {
					continue
				}
				seenModels[key] = struct{}{}
				models = append(models, buildYucoreMediaCatalogModel(ability.Model, kind, capability, configured, groupRatio))
			}
		}
		if len(models) == 0 {
			continue
		}
		sort.SliceStable(models, func(i, j int) bool {
			if models[i].Id == models[j].Id {
				return models[i].Kind < models[j].Kind
			}
			return models[i].Id < models[j].Id
		})
		catalog.Groups = append(catalog.Groups, YucoreMediaCatalogGroup{
			Id:          groupID,
			Description: usableGroups[groupID],
			Ratio:       groupRatio,
			Models:      models,
		})
	}
	if len(catalog.Groups) > 0 {
		catalog.DefaultGroup = catalog.Groups[0].Id
	}
	return catalog, nil
}

func ResolveYucoreMediaSelection(userID int, group string, modelID string, kind string) (string, YucoreMediaCatalogModel, error) {
	catalog, err := BuildYucoreMediaCatalog(userID)
	if err != nil {
		return "", YucoreMediaCatalogModel{}, err
	}
	group = strings.TrimSpace(group)
	if group == "" {
		group = catalog.DefaultGroup
	}
	modelID = strings.TrimSpace(modelID)
	kind = strings.ToLower(strings.TrimSpace(kind))
	for _, catalogGroup := range catalog.Groups {
		if catalogGroup.Id != group {
			continue
		}
		for _, catalogModel := range catalogGroup.Models {
			if catalogModel.Id == modelID && catalogModel.Kind == kind {
				return group, catalogModel, nil
			}
		}
		return "", YucoreMediaCatalogModel{}, fmt.Errorf("model %s is not available for %s generation in group %s", modelID, kind, group)
	}
	if group == "" {
		return "", YucoreMediaCatalogModel{}, errors.New("no image or video models are available")
	}
	return "", YucoreMediaCatalogModel{}, fmt.Errorf("group %s is not available for media generation", group)
}
