package service

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/model"
)

const (
	yucoreMediaUploadReferencePrefix  = "/api/yucore/media/uploads/"
	maxYucoreMediaReferenceValueSize  = 512 * 1024
	maxYucoreMediaOpaqueReferenceSize = 160
)

type YucoreMediaRequestOptions struct {
	Mode           string
	Count          int
	Duration       *int
	Resolution     string
	AspectRatio    string
	GenerateAudio  *bool
	Seed           *int64
	NegativePrompt *string
	ReferenceMode  string
	References     []model.YucoreMediaReferenceInput
}

func NormalizeYucoreMediaRequest(selected YucoreMediaCatalogModel, options YucoreMediaRequestOptions) (YucoreMediaRequestOptions, error) {
	normalized := YucoreMediaRequestOptions{
		Mode:          strings.ToLower(strings.TrimSpace(options.Mode)),
		Count:         options.Count,
		Resolution:    strings.TrimSpace(options.Resolution),
		AspectRatio:   strings.TrimSpace(options.AspectRatio),
		ReferenceMode: strings.ToLower(strings.TrimSpace(options.ReferenceMode)),
		References:    make([]model.YucoreMediaReferenceInput, len(options.References)),
	}
	if options.Duration != nil {
		value := *options.Duration
		normalized.Duration = &value
	}
	if options.GenerateAudio != nil {
		value := *options.GenerateAudio
		normalized.GenerateAudio = &value
	}
	if options.Seed != nil {
		value := *options.Seed
		normalized.Seed = &value
	}
	if options.NegativePrompt != nil {
		value := strings.TrimSpace(*options.NegativePrompt)
		if value != "" {
			normalized.NegativePrompt = &value
		}
	}
	for index, reference := range options.References {
		normalized.References[index] = model.YucoreMediaReferenceInput{
			Role:     strings.ToLower(strings.TrimSpace(reference.Role)),
			URL:      reference.URL,
			MimeType: strings.ToLower(strings.TrimSpace(reference.MimeType)),
		}
		if reference.DurationMS != nil {
			value := *reference.DurationMS
			normalized.References[index].DurationMS = &value
		}
	}

	resolvedMode, resolvedCount, err := ValidateYucoreMediaRequest(selected, normalized.Mode, normalized.Count, 0)
	if err != nil {
		return YucoreMediaRequestOptions{}, err
	}
	normalized.Mode = resolvedMode
	normalized.Count = resolvedCount

	capability := model.YucoreMediaModelCapability{}
	if selected.capability != nil {
		capability = cloneYucoreMediaCapability(*selected.capability)
	} else {
		_, capabilities := model.GetYucoreMediaCatalogSettings()
		capability, _ = yucoreMediaCapabilityForModel(capabilities, selected.Id)
	}

	durations := capability.Durations
	if len(durations) == 0 {
		durations = selected.Durations
	}
	fixedDuration := capability.FixedDurationSeconds
	if fixedDuration > 0 {
		if normalized.Duration != nil && *normalized.Duration != fixedDuration {
			return YucoreMediaRequestOptions{}, fmt.Errorf("model %s requires duration %d", selected.Id, fixedDuration)
		}
		value := fixedDuration
		normalized.Duration = &value
	} else if normalized.Duration != nil {
		if !yucoreMediaIntAllowed(*normalized.Duration, durations) {
			return YucoreMediaRequestOptions{}, fmt.Errorf("model %s does not support duration %d", selected.Id, *normalized.Duration)
		}
	} else if len(durations) > 0 {
		value := durations[0]
		normalized.Duration = &value
	}

	resolutions := capability.Resolutions
	if len(resolutions) == 0 {
		resolutions = selected.Resolutions
	}
	if normalized.Resolution == "" && len(resolutions) > 0 {
		normalized.Resolution = resolutions[0]
	} else if normalized.Resolution != "" && len(resolutions) > 0 {
		canonical, ok := yucoreMediaCanonicalString(normalized.Resolution, resolutions)
		if !ok {
			return YucoreMediaRequestOptions{}, fmt.Errorf("model %s does not support resolution %s", selected.Id, normalized.Resolution)
		}
		normalized.Resolution = canonical
	}

	aspectRatios := capability.AspectRatios
	if len(aspectRatios) == 0 {
		aspectRatios = selected.AspectRatios
	}
	if normalized.AspectRatio != "" && len(aspectRatios) > 0 {
		canonical, ok := yucoreMediaCanonicalString(normalized.AspectRatio, aspectRatios)
		if !ok {
			return YucoreMediaRequestOptions{}, fmt.Errorf("model %s does not support aspect ratio %s", selected.Id, normalized.AspectRatio)
		}
		normalized.AspectRatio = canonical
	}

	supportsAudio := capability.SupportsAudio || selected.SupportsAudio || yucoreMediaCapabilityAllows(capability, "generate_audio")
	if normalized.GenerateAudio != nil && !supportsAudio {
		return YucoreMediaRequestOptions{}, fmt.Errorf("model %s does not support generate audio", selected.Id)
	}
	supportsSeed := capability.SupportsSeed || selected.SupportsSeed || yucoreMediaCapabilityAllows(capability, "seed")
	if normalized.Seed != nil && !supportsSeed {
		return YucoreMediaRequestOptions{}, fmt.Errorf("model %s does not support seed", selected.Id)
	}
	if normalized.NegativePrompt != nil && !yucoreMediaCapabilityAllows(capability, "negative_prompt") {
		return YucoreMediaRequestOptions{}, fmt.Errorf("model %s does not support negative prompt", selected.Id)
	}

	imageCount := 0
	videoCount := 0
	audioCount := 0
	firstFrameCount := 0
	lastFrameCount := 0
	for index := range normalized.References {
		reference := &normalized.References[index]
		switch reference.Role {
		case "image", "video", "audio", "first_frame", "last_frame":
		default:
			return YucoreMediaRequestOptions{}, fmt.Errorf("media reference role %s is invalid", reference.Role)
		}
		referenceURL, err := normalizeYucoreMediaReferenceValue(reference.Role, reference.URL)
		if err != nil {
			return YucoreMediaRequestOptions{}, err
		}
		reference.URL = referenceURL
		if reference.DurationMS != nil && *reference.DurationMS < 0 {
			return YucoreMediaRequestOptions{}, fmt.Errorf("media reference duration must not be negative")
		}
		switch reference.Role {
		case "image":
			imageCount++
		case "video":
			videoCount++
			if limit := selected.InputLimits.MaxReferenceVideoDurationMS; limit > 0 && reference.DurationMS != nil && *reference.DurationMS > limit {
				return YucoreMediaRequestOptions{}, fmt.Errorf("model %s reference video duration exceeds %d ms", selected.Id, limit)
			}
		case "audio":
			audioCount++
			if limit := selected.InputLimits.MaxReferenceAudioDurationMS; limit > 0 && reference.DurationMS != nil && *reference.DurationMS > limit {
				return YucoreMediaRequestOptions{}, fmt.Errorf("model %s reference audio duration exceeds %d ms", selected.Id, limit)
			}
		case "first_frame":
			firstFrameCount++
			imageCount++
		case "last_frame":
			lastFrameCount++
			imageCount++
		}
	}

	if normalized.ReferenceMode == "" {
		switch {
		case firstFrameCount > 0 || lastFrameCount > 0:
			normalized.ReferenceMode = "frames"
		case len(normalized.References) > 0:
			normalized.ReferenceMode = "media"
		default:
			normalized.ReferenceMode = "text"
		}
	}
	switch normalized.ReferenceMode {
	case "text":
		if len(normalized.References) != 0 {
			return YucoreMediaRequestOptions{}, fmt.Errorf("text reference mode does not accept references")
		}
	case "frames":
		if firstFrameCount != 1 || lastFrameCount != 1 {
			return YucoreMediaRequestOptions{}, fmt.Errorf("frames reference mode requires exactly one first_frame and last_frame")
		}
		if imageCount != 2 || videoCount > 0 || audioCount > 0 {
			return YucoreMediaRequestOptions{}, fmt.Errorf("frame references cannot be mixed with media references")
		}
	case "media":
		if firstFrameCount > 0 || lastFrameCount > 0 {
			return YucoreMediaRequestOptions{}, fmt.Errorf("frame references cannot be mixed with media references")
		}
	default:
		return YucoreMediaRequestOptions{}, fmt.Errorf("media reference mode %s is invalid", normalized.ReferenceMode)
	}
	if normalized.ReferenceMode != "text" && len(selected.ReferenceModes) > 0 {
		if _, ok := yucoreMediaCanonicalString(normalized.ReferenceMode, selected.ReferenceModes); !ok {
			return YucoreMediaRequestOptions{}, fmt.Errorf("model %s does not support reference mode %s", selected.Id, normalized.ReferenceMode)
		}
	}

	limits := selected.InputLimits
	if imageCount > limits.MaxReferenceImages {
		return YucoreMediaRequestOptions{}, fmt.Errorf("model %s supports at most %d reference image(s)", selected.Id, limits.MaxReferenceImages)
	}
	if videoCount > limits.MaxReferenceVideos {
		return YucoreMediaRequestOptions{}, fmt.Errorf("model %s supports at most %d reference video(s)", selected.Id, limits.MaxReferenceVideos)
	}
	if audioCount > limits.MaxReferenceAudios {
		return YucoreMediaRequestOptions{}, fmt.Errorf("model %s supports at most %d reference audio(s)", selected.Id, limits.MaxReferenceAudios)
	}
	if limits.MaxReferences > 0 && len(normalized.References) > limits.MaxReferences {
		return YucoreMediaRequestOptions{}, fmt.Errorf("model %s supports at most %d total references", selected.Id, limits.MaxReferences)
	}

	hasPrimaryImageParameter := yucoreMediaCapabilityAllowsAny(capability, "image", "images", "image_url", "image_urls")
	hasVideoReferenceParameter := videoCount > 0 && yucoreMediaCapabilityAllowsAny(capability, "video", "reference_videos")
	hasAudioReferenceParameter := audioCount > 0 && yucoreMediaCapabilityAllowsAny(capability, "audio", "reference_audios")
	if imageCount == 0 && hasPrimaryImageParameter && (hasVideoReferenceParameter || hasAudioReferenceParameter) {
		return YucoreMediaRequestOptions{}, fmt.Errorf("model %s requires a primary image with video or audio references", selected.Id)
	}

	return normalized, nil
}

func normalizeYucoreMediaReferenceValue(role string, value string) (string, error) {
	if yucoreMediaContainsControl(value) {
		return "", fmt.Errorf("media reference value contains a control character")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("media reference URL is required")
	}
	if strings.Contains(value, "\\") {
		return "", fmt.Errorf("media reference value is invalid")
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return "", fmt.Errorf("media reference value is invalid")
		}
	}

	lowerValue := strings.ToLower(value)
	if strings.HasPrefix(lowerValue, "data:") {
		if role != "image" && role != "first_frame" && role != "last_frame" {
			return "", fmt.Errorf("media reference role %s does not accept a data URL", role)
		}
		if len(value) > maxYucoreMediaReferenceValueSize {
			return "", fmt.Errorf("media reference data URL is too large")
		}
		comma := strings.IndexByte(value, ',')
		if comma < 0 {
			return "", fmt.Errorf("media reference must be an image data URL")
		}
		mediaType := strings.TrimSpace(strings.SplitN(lowerValue[len("data:"):comma], ";", 2)[0])
		if !strings.HasPrefix(mediaType, "image/") || len(mediaType) == len("image/") {
			return "", fmt.Errorf("media reference must be an image data URL")
		}
		return value, nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("media reference value is invalid")
	}
	decodedQuery, err := url.QueryUnescape(parsed.RawQuery)
	if err != nil {
		return "", fmt.Errorf("media reference value is invalid")
	}
	if yucoreMediaContainsControl(parsed.Path) || yucoreMediaContainsControl(decodedQuery) || yucoreMediaContainsControl(parsed.Fragment) {
		return "", fmt.Errorf("media reference value contains a control character")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "" {
		if scheme != "http" && scheme != "https" {
			return "", fmt.Errorf("media reference value uses an unsupported scheme")
		}
		if parsed.User != nil {
			return "", fmt.Errorf("media reference URL must not contain userinfo")
		}
		if parsed.Host == "" || !yucoreMediaValidHost(parsed.Hostname()) {
			return "", fmt.Errorf("media reference URL requires a valid host")
		}
		return value, nil
	}

	if strings.HasPrefix(value, "/") {
		if parsed.Host != "" || parsed.User != nil || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, yucoreMediaUploadReferencePrefix) {
			return "", fmt.Errorf("media reference value is not an allowed upload path")
		}
		if strings.Contains(value, "\\") || path.Clean(parsed.Path) != parsed.Path {
			return "", fmt.Errorf("media reference upload path contains traversal")
		}
		remainder := strings.TrimPrefix(parsed.Path, yucoreMediaUploadReferencePrefix)
		segments := strings.Split(remainder, "/")
		if len(segments) != 2 || segments[0] == "" || segments[1] == "" || segments[1] == "." || segments[1] == ".." {
			return "", fmt.Errorf("media reference upload path is invalid")
		}
		ownerID, ownerErr := strconv.Atoi(segments[0])
		if ownerErr != nil || ownerID <= 0 {
			return "", fmt.Errorf("media reference upload path has an invalid owner")
		}
		return value, nil
	}

	if len(value) > maxYucoreMediaOpaqueReferenceSize {
		return "", fmt.Errorf("media reference value is invalid")
	}
	if strings.HasPrefix(value, "ref_") {
		parts := strings.Split(value, "_")
		if len(parts) != 3 || len(parts[1]) < 10 || len(parts[1]) > 13 || len(parts[2]) != 10 ||
			!yucoreMediaASCIIString(parts[1], true) || !yucoreMediaASCIIString(parts[2], false) {
			return "", fmt.Errorf("media reference value is invalid")
		}
		return value, nil
	}
	if strings.HasPrefix(value, "asset_") {
		identifier := strings.TrimPrefix(value, "asset_")
		if len(identifier) == 0 || len(identifier) > 20 || !yucoreMediaASCIIString(identifier, true) {
			return "", fmt.Errorf("media reference value is invalid")
		}
		return value, nil
	}
	return "", fmt.Errorf("media reference value is invalid")
}

func yucoreMediaContainsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func yucoreMediaValidHost(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" || len(host) > 253 {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func yucoreMediaASCIIString(value string, digitsOnly bool) bool {
	for _, character := range value {
		if character >= '0' && character <= '9' {
			continue
		}
		if !digitsOnly && ((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')) {
			continue
		}
		return false
	}
	return true
}

func yucoreMediaIntAllowed(value int, allowed []int) bool {
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}

func yucoreMediaCanonicalString(value string, allowed []string) (string, bool) {
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(value)) {
			return strings.TrimSpace(candidate), true
		}
	}
	return "", false
}

func yucoreMediaCapabilityAllowsAny(capability model.YucoreMediaModelCapability, parameters ...string) bool {
	for _, parameter := range parameters {
		if yucoreMediaCapabilityAllows(capability, parameter) {
			return true
		}
	}
	return false
}
