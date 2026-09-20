package service

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	groupProtocolOpenAI = "openai"
	groupProtocolClaude = "claude"
	groupProtocolGemini = "gemini"
	groupProtocolImage  = "image"
	groupProtocolVideo  = "video"
	groupProtocolOther  = "other"
)

var groupProtocolOrder = []string{
	groupProtocolOpenAI,
	groupProtocolClaude,
	groupProtocolGemini,
	groupProtocolImage,
	groupProtocolVideo,
}

type GroupProtocolMetadata struct {
	Protocols     []string `json:"protocols"`
	EndpointPaths []string `json:"endpoint_paths"`
}

type groupProtocolAccumulator map[string]map[string]struct{}

func (a groupProtocolAccumulator) add(protocol string, endpointPath string) {
	if protocol == "" {
		return
	}
	if _, ok := a[protocol]; !ok {
		a[protocol] = make(map[string]struct{})
	}
	if endpointPath != "" {
		a[protocol][endpointPath] = struct{}{}
	}
}

func (a groupProtocolAccumulator) metadata() GroupProtocolMetadata {
	metadata := GroupProtocolMetadata{
		Protocols:     make([]string, 0, len(a)),
		EndpointPaths: make([]string, 0),
	}
	seenPaths := make(map[string]struct{})
	for _, protocol := range groupProtocolOrder {
		paths, ok := a[protocol]
		if !ok {
			continue
		}
		metadata.Protocols = append(metadata.Protocols, protocol)
		sortedPaths := make([]string, 0, len(paths))
		for path := range paths {
			sortedPaths = append(sortedPaths, path)
		}
		sort.Strings(sortedPaths)
		for _, path := range sortedPaths {
			if _, exists := seenPaths[path]; exists {
				continue
			}
			seenPaths[path] = struct{}{}
			metadata.EndpointPaths = append(metadata.EndpointPaths, path)
		}
	}
	if otherPaths, ok := a[groupProtocolOther]; ok {
		sortedPaths := make([]string, 0, len(otherPaths))
		for path := range otherPaths {
			sortedPaths = append(sortedPaths, path)
		}
		sort.Strings(sortedPaths)
		for _, path := range sortedPaths {
			if _, exists := seenPaths[path]; exists {
				continue
			}
			seenPaths[path] = struct{}{}
			metadata.EndpointPaths = append(metadata.EndpointPaths, path)
		}
	}
	return metadata
}

func BuildGroupProtocolMetadata(abilities []model.AbilityWithChannel) map[string]GroupProtocolMetadata {
	capabilitiesByGroup := make(map[string]groupProtocolAccumulator)
	processedAdvancedChannels := make(map[string]map[int]struct{})
	for _, ability := range abilities {
		groupName := strings.TrimSpace(ability.Group)
		if groupName == "" {
			continue
		}
		if _, ok := capabilitiesByGroup[groupName]; !ok {
			capabilitiesByGroup[groupName] = make(groupProtocolAccumulator)
		}

		if ability.ChannelType == constant.ChannelTypeAdvancedCustom {
			if _, ok := processedAdvancedChannels[groupName]; !ok {
				processedAdvancedChannels[groupName] = make(map[int]struct{})
			}
			if _, processed := processedAdvancedChannels[groupName][ability.ChannelId]; processed {
				continue
			}
			processedAdvancedChannels[groupName][ability.ChannelId] = struct{}{}
			settings := dto.ChannelOtherSettings{}
			if common.UnmarshalJsonStr(ability.ChannelSettings, &settings) != nil || settings.AdvancedCustom == nil {
				continue
			}
			for _, route := range settings.AdvancedCustom.Routes {
				path := strings.TrimSpace(route.IncomingPath)
				protocol := groupProtocolForPath(path)
				if protocol == "" {
					protocol = groupProtocolOther
				}
				capabilitiesByGroup[groupName].add(protocol, path)
			}
			continue
		}
		if common.IsVideoGenerationModel(ability.ChannelType, ability.Model) {
			capabilitiesByGroup[groupName].add(groupProtocolVideo, "/v1/videos")
			continue
		}
		if isGroupProtocolImageModel(ability.Model) {
			capabilitiesByGroup[groupName].add(groupProtocolImage, "/v1/images/generations")
		}

		for _, endpointType := range common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model) {
			protocol := groupProtocolForEndpointType(endpointType)
			endpointPath := ""
			if info, ok := common.GetDefaultEndpointInfo(endpointType); ok {
				endpointPath = info.Path
			}
			capabilitiesByGroup[groupName].add(protocol, endpointPath)
		}
	}

	metadataByGroup := make(map[string]GroupProtocolMetadata, len(capabilitiesByGroup))
	for groupName, accumulator := range capabilitiesByGroup {
		metadata := accumulator.metadata()
		if len(metadata.Protocols) > 0 || len(metadata.EndpointPaths) > 0 {
			metadataByGroup[groupName] = metadata
		}
	}
	return metadataByGroup
}

func isGroupProtocolImageModel(modelName string) bool {
	if common.IsImageGenerationModel(modelName) {
		return true
	}

	modelName = strings.ToLower(strings.TrimSpace(modelName))
	return strings.Contains(modelName, "grok-imagine-image") ||
		strings.Contains(modelName, "jimeng_high_aes_general") ||
		strings.Contains(modelName, "seedream")
}

func MergeGroupProtocolMetadata(groupNames []string, metadataByGroup map[string]GroupProtocolMetadata) GroupProtocolMetadata {
	merged := make(groupProtocolAccumulator)
	for _, groupName := range groupNames {
		metadata, ok := metadataByGroup[groupName]
		if !ok {
			continue
		}
		for _, path := range metadata.EndpointPaths {
			protocol := groupProtocolForPath(path)
			if protocol == "" {
				protocol = groupProtocolOther
			}
			merged.add(protocol, path)
		}
		for _, protocol := range metadata.Protocols {
			merged.add(protocol, "")
		}
	}
	return merged.metadata()
}

func groupProtocolForEndpointType(endpointType constant.EndpointType) string {
	switch endpointType {
	case constant.EndpointTypeAnthropic:
		return groupProtocolClaude
	case constant.EndpointTypeGemini:
		return groupProtocolGemini
	case constant.EndpointTypeImageGeneration:
		return groupProtocolImage
	case constant.EndpointTypeOpenAIVideo:
		return groupProtocolVideo
	case constant.EndpointTypeOpenAI,
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeOpenAIResponseCompact,
		constant.EndpointTypeJinaRerank,
		constant.EndpointTypeEmbeddings:
		return groupProtocolOpenAI
	default:
		return ""
	}
}

func groupProtocolForPath(path string) string {
	path = strings.ToLower(strings.TrimSpace(path))
	switch {
	case path == "/v1/messages":
		return groupProtocolClaude
	case isGeminiModelPath(path, ":generatecontent"),
		isGeminiModelPath(path, ":streamgeneratecontent"),
		isGeminiModelPath(path, ":embedcontent"),
		isGeminiModelPath(path, ":batchembedcontents"):
		return groupProtocolGemini
	case strings.HasPrefix(path, "/v1/images/"):
		return groupProtocolImage
	case path == "/v1/videos",
		path == "/v1/videos/generations",
		strings.HasPrefix(path, "/v1/videos/"):
		return groupProtocolVideo
	case path == "/v1/chat/completions",
		path == "/v1/completions",
		path == "/v1/responses",
		path == "/v1/responses/compact",
		path == "/v1/embeddings",
		path == "/v1/rerank",
		path == "/v1/audio/speech",
		path == "/v1/audio/transcriptions",
		path == "/v1/audio/translations",
		path == "/v1/realtime":
		return groupProtocolOpenAI
	default:
		return ""
	}
}

func isGeminiModelPath(path string, suffix string) bool {
	return (strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/")) &&
		strings.HasSuffix(path, suffix)
}
