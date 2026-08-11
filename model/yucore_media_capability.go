package model

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	YucoreMediaAvailabilityEnabled = "enabled"
	YucoreMediaAvailabilityProbe   = "probe"

	YucoreMediaPricingPerCall   = "per_call"
	YucoreMediaPricingPerSecond = "per_second"

	yucoreMediaTransportSyncImage = "sync-image"
	yucoreMediaTransportAsyncTask = "async-task"

	yucoreMediaDurationPolicyDuration = "duration"
	yucoreMediaDurationPolicySeconds  = "seconds"
	yucoreMediaDurationPolicyFixed    = "fixed"
	yucoreMediaDurationPolicyNone     = "none"
)

type YucoreMediaReferenceLimits struct {
	Images             int `json:"images,omitempty"`
	Videos             int `json:"videos,omitempty"`
	Audios             int `json:"audios,omitempty"`
	Total              int `json:"total,omitempty"`
	MaxVideoDurationMS int `json:"max_video_duration_ms,omitempty"`
	MaxAudioDurationMS int `json:"max_audio_duration_ms,omitempty"`
}

type YucoreMediaReferenceInput struct {
	Role       string `json:"role"`
	URL        string `json:"url"`
	MimeType   string `json:"mime_type,omitempty"`
	DurationMS *int   `json:"duration_ms,omitempty"`
}

type YucoreMediaModelCapability struct {
	Model                  string                     `json:"model,omitempty"`
	UpstreamModel          string                     `json:"upstream_model,omitempty"`
	Kind                   string                     `json:"kind,omitempty"`
	Family                 string                     `json:"family,omitempty"`
	Availability           string                     `json:"availability,omitempty"`
	PricingUnit            string                     `json:"pricing_unit,omitempty"`
	UpstreamCost           float64                    `json:"upstream_cost,omitempty"`
	Transport              string                     `json:"transport,omitempty"`
	CreatePath             string                     `json:"create_path,omitempty"`
	EditPath               string                     `json:"edit_path,omitempty"`
	StatusPath             string                     `json:"status_path,omitempty"`
	ContentPath            string                     `json:"content_path,omitempty"`
	CancelPath             string                     `json:"cancel_path,omitempty"`
	DurationPolicy         string                     `json:"duration_policy,omitempty"`
	FixedDurationSeconds   int                        `json:"fixed_duration_seconds,omitempty"`
	Durations              []int                      `json:"durations,omitempty"`
	Resolutions            []string                   `json:"resolutions,omitempty"`
	AspectRatios           []string                   `json:"aspect_ratios,omitempty"`
	ReferenceModes         []string                   `json:"reference_modes,omitempty"`
	ReferenceLimits        YucoreMediaReferenceLimits `json:"reference_limits,omitempty"`
	SupportsAudio          bool                       `json:"supports_audio,omitempty"`
	SupportsSeed           bool                       `json:"supports_seed,omitempty"`
	PollIntervalSeconds    int                        `json:"poll_interval_seconds,omitempty"`
	MaxPollDurationSeconds int                        `json:"max_poll_duration_seconds,omitempty"`
	MaxReferenceImages     int                        `json:"max_reference_images,omitempty"`
	AllowedParameters      []string                   `json:"allowed_parameters,omitempty"`
	TerminalSuccessStates  []string                   `json:"terminal_success_states,omitempty"`
	TerminalFailureStates  []string                   `json:"terminal_failure_states,omitempty"`
	ResponseFormat         string                     `json:"response_format,omitempty"`
	Notes                  []string                   `json:"notes,omitempty"`
}

//go:embed yucore_media_cangyuan_catalog.json
var cangyuanMediaCatalogJSON []byte

func cloneYucoreMediaModelCapabilities(source map[string]YucoreMediaModelCapability) map[string]YucoreMediaModelCapability {
	if source == nil {
		return nil
	}
	cloned := make(map[string]YucoreMediaModelCapability, len(source))
	for modelID, capability := range source {
		capability.Durations = append([]int(nil), capability.Durations...)
		capability.Resolutions = append([]string(nil), capability.Resolutions...)
		capability.AspectRatios = append([]string(nil), capability.AspectRatios...)
		capability.ReferenceModes = append([]string(nil), capability.ReferenceModes...)
		capability.AllowedParameters = append([]string(nil), capability.AllowedParameters...)
		capability.TerminalSuccessStates = append([]string(nil), capability.TerminalSuccessStates...)
		capability.TerminalFailureStates = append([]string(nil), capability.TerminalFailureStates...)
		capability.Notes = append([]string(nil), capability.Notes...)
		cloned[modelID] = capability
	}
	return cloned
}

func decodeYucoreMediaCapabilityDocument(raw []byte) (map[string]map[string]any, error) {
	if err := common.ValidateJSONTopLevelObjectUniqueKeys(raw); err != nil {
		var duplicate *common.DuplicateJSONTopLevelKeyError
		if errors.As(err, &duplicate) {
			return nil, fmt.Errorf("YuCore media model capabilities contain duplicate model %s", strings.TrimSpace(duplicate.Key))
		}
		return nil, errors.New("YuCore media model capabilities must be a JSON object or array")
	}
	var document any
	if err := common.Unmarshal(raw, &document); err != nil {
		return nil, errors.New("YuCore media model capabilities must be a JSON object or array")
	}

	rows := make(map[string]map[string]any)
	addRow := func(modelID string, value any) error {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return errors.New("YuCore media model capabilities contain an empty model ID")
		}
		if _, exists := rows[modelID]; exists {
			return fmt.Errorf("YuCore media model capabilities contain duplicate model %s", modelID)
		}
		row, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("YuCore media model %s capability must be an object", modelID)
		}
		rowCopy := make(map[string]any, len(row)+1)
		for key, field := range row {
			rowCopy[key] = field
		}
		rowCopy["model"] = modelID
		if legacyLimit, hasLegacy := rowCopy["max_reference_images"]; hasLegacy {
			if _, hasRicherLimits := rowCopy["reference_limits"]; !hasRicherLimits {
				rowCopy["reference_limits"] = map[string]any{"images": legacyLimit}
			}
		}
		rows[modelID] = rowCopy
		return nil
	}

	switch value := document.(type) {
	case map[string]any:
		for modelID, row := range value {
			if err := addRow(modelID, row); err != nil {
				return nil, err
			}
		}
	case []any:
		for _, item := range value {
			row, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("YuCore media model capability array entries must be objects")
			}
			modelID, _ := row["model"].(string)
			if err := addRow(modelID, row); err != nil {
				return nil, err
			}
		}
	default:
		return nil, errors.New("YuCore media model capabilities must be a JSON object or array")
	}
	return rows, nil
}

func decodeYucoreMediaModelCapabilities(raw []byte) (map[string]YucoreMediaModelCapability, error) {
	rows, err := decodeYucoreMediaCapabilityDocument(raw)
	if err != nil {
		return nil, err
	}
	capabilities := make(map[string]YucoreMediaModelCapability, len(rows))
	for modelID, row := range rows {
		encoded, err := common.Marshal(row)
		if err != nil {
			return nil, err
		}
		var capability YucoreMediaModelCapability
		if err := common.Unmarshal(encoded, &capability); err != nil {
			return nil, fmt.Errorf("YuCore media model %s has invalid capability fields: %w", modelID, err)
		}
		capability.Model = modelID
		capabilities[modelID] = capability
	}
	return capabilities, nil
}

func parseYucoreMediaModelCapabilities(raw string) map[string]YucoreMediaModelCapability {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	capabilities, err := decodeYucoreMediaModelCapabilities([]byte(raw))
	if err != nil {
		return nil
	}
	return cloneYucoreMediaModelCapabilities(capabilities)
}

func mergeYucoreMediaModelCapabilities(base map[string]YucoreMediaModelCapability, raw string) map[string]YucoreMediaModelCapability {
	merged := cloneYucoreMediaModelCapabilities(base)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return merged
	}
	overrides, err := decodeYucoreMediaCapabilityDocument([]byte(raw))
	if err != nil {
		return merged
	}
	for modelID, override := range overrides {
		row := make(map[string]any)
		if existing, ok := merged[modelID]; ok {
			encoded, err := common.Marshal(existing)
			if err != nil || common.Unmarshal(encoded, &row) != nil {
				continue
			}
		}
		for key, value := range override {
			if key == "reference_limits" {
				overrideLimits, overrideIsObject := value.(map[string]any)
				baseLimits, baseIsObject := row[key].(map[string]any)
				if overrideIsObject && baseIsObject {
					mergedLimits := make(map[string]any, len(baseLimits)+len(overrideLimits))
					for limitName, limitValue := range baseLimits {
						mergedLimits[limitName] = limitValue
					}
					for limitName, limitValue := range overrideLimits {
						mergedLimits[limitName] = limitValue
					}
					value = mergedLimits
				}
			}
			row[key] = value
		}
		encoded, err := common.Marshal(row)
		if err != nil {
			continue
		}
		var capability YucoreMediaModelCapability
		if err := common.Unmarshal(encoded, &capability); err != nil {
			continue
		}
		capability.Model = modelID
		merged[modelID] = capability
	}
	return cloneYucoreMediaModelCapabilities(merged)
}

func loadCangyuanMediaCatalog() (map[string]YucoreMediaModelCapability, error) {
	capabilities, err := decodeYucoreMediaModelCapabilities(cangyuanMediaCatalogJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid embedded Cangyuan media catalog: %w", err)
	}
	if err := validateYucoreMediaCapabilities(capabilities); err != nil {
		return nil, fmt.Errorf("invalid embedded Cangyuan media catalog: %w", err)
	}
	return cloneYucoreMediaModelCapabilities(capabilities), nil
}

func validateYucoreMediaModelCapabilities(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	capabilities, err := decodeYucoreMediaModelCapabilities([]byte(raw))
	if err != nil {
		return err
	}
	return validateYucoreMediaCapabilities(capabilities)
}

func validateYucoreMediaCapabilities(capabilities map[string]YucoreMediaModelCapability) error {
	for modelID, capability := range capabilities {
		transport := strings.ToLower(strings.TrimSpace(capability.Transport))
		switch transport {
		case "", yucoreMediaTransportSyncImage, yucoreMediaTransportAsyncTask, "async-image-task", "async-video-task":
		default:
			return fmt.Errorf("YuCore media model %s has an invalid transport", modelID)
		}
		policy := strings.ToLower(strings.TrimSpace(capability.DurationPolicy))
		switch policy {
		case "", yucoreMediaDurationPolicyDuration, yucoreMediaDurationPolicySeconds, yucoreMediaDurationPolicyFixed, yucoreMediaDurationPolicyNone:
		default:
			return fmt.Errorf("YuCore media model %s has an invalid duration policy", modelID)
		}
		if policy == yucoreMediaDurationPolicyFixed && capability.FixedDurationSeconds <= 0 {
			return fmt.Errorf("YuCore media model %s requires a positive fixed duration", modelID)
		}
		switch strings.ToLower(strings.TrimSpace(capability.Availability)) {
		case "", YucoreMediaAvailabilityEnabled, YucoreMediaAvailabilityProbe:
		default:
			return fmt.Errorf("YuCore media model %s has an invalid availability", modelID)
		}
		switch strings.ToLower(strings.TrimSpace(capability.PricingUnit)) {
		case "", YucoreMediaPricingPerCall, YucoreMediaPricingPerSecond:
		default:
			return fmt.Errorf("YuCore media model %s has an invalid pricing unit", modelID)
		}
		if capability.UpstreamCost < 0 {
			return fmt.Errorf("YuCore media model %s has an invalid upstream cost", modelID)
		}
		if capability.PollIntervalSeconds < 0 || capability.PollIntervalSeconds > 3600 {
			return fmt.Errorf("YuCore media model %s has an invalid poll interval", modelID)
		}
		if capability.MaxPollDurationSeconds < 0 || capability.MaxPollDurationSeconds > 86400 ||
			(capability.MaxPollDurationSeconds > 0 && capability.MaxPollDurationSeconds < capability.PollIntervalSeconds) {
			return fmt.Errorf("YuCore media model %s has an invalid maximum poll duration", modelID)
		}
		for label, endpointPath := range map[string]string{
			"create":  capability.CreatePath,
			"edit":    capability.EditPath,
			"status":  capability.StatusPath,
			"content": capability.ContentPath,
			"cancel":  capability.CancelPath,
		} {
			endpointPath = strings.TrimSpace(endpointPath)
			if endpointPath == "" {
				continue
			}
			if !strings.HasPrefix(endpointPath, "/") {
				return fmt.Errorf("YuCore media model %s %s path must start with /", modelID, label)
			}
			if label != "create" && label != "edit" && !strings.Contains(endpointPath, "{task_id}") {
				return fmt.Errorf("YuCore media model %s %s path must contain {task_id}", modelID, label)
			}
		}

		limits := capability.ReferenceLimits
		if limits.Images < 0 || limits.Images > 32 || capability.MaxReferenceImages < 0 || capability.MaxReferenceImages > 32 {
			return fmt.Errorf("YuCore media model %s has an invalid reference image limit", modelID)
		}
		if limits.Videos < 0 || limits.Videos > 32 {
			return fmt.Errorf("YuCore media model %s has an invalid reference video limit", modelID)
		}
		if limits.Audios < 0 || limits.Audios > 32 {
			return fmt.Errorf("YuCore media model %s has an invalid reference audio limit", modelID)
		}
		if limits.Total < 0 || limits.Total > 32 {
			return fmt.Errorf("YuCore media model %s has an invalid reference total limit", modelID)
		}
		if limits.MaxVideoDurationMS < 0 || limits.MaxAudioDurationMS < 0 {
			return fmt.Errorf("YuCore media model %s has an invalid reference duration limit", modelID)
		}
		if capability.MaxReferenceImages > 0 && limits.Images > 0 && capability.MaxReferenceImages != limits.Images {
			return fmt.Errorf("YuCore media model %s has conflicting reference image limits", modelID)
		}
		if strings.EqualFold(strings.TrimSpace(capability.Kind), "image") && limits.Videos > 0 {
			return fmt.Errorf("YuCore media image model cannot accept video references: %s", modelID)
		}
		if err := validateYucoreMediaTerminalStates(modelID, capability.TerminalSuccessStates, capability.TerminalFailureStates); err != nil {
			return err
		}
	}
	return nil
}

func validateYucoreMediaTerminalStates(modelID string, successStates []string, failureStates []string) error {
	success := make(map[string]struct{}, len(successStates))
	for _, state := range successStates {
		state = strings.ToLower(strings.TrimSpace(state))
		if state == "" {
			return fmt.Errorf("YuCore media model %s has invalid terminal success states", modelID)
		}
		if _, duplicate := success[state]; duplicate {
			return fmt.Errorf("YuCore media model %s has duplicate terminal success state %s", modelID, state)
		}
		success[state] = struct{}{}
	}
	failure := make(map[string]struct{}, len(failureStates))
	for _, state := range failureStates {
		state = strings.ToLower(strings.TrimSpace(state))
		if state == "" {
			return fmt.Errorf("YuCore media model %s has invalid terminal failure states", modelID)
		}
		if _, overlaps := success[state]; overlaps {
			return fmt.Errorf("YuCore media model %s terminal states overlap", modelID)
		}
		if _, duplicate := failure[state]; duplicate {
			return fmt.Errorf("YuCore media model %s has duplicate terminal failure state %s", modelID, state)
		}
		failure[state] = struct{}{}
	}
	return nil
}
