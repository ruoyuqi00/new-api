package xai

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var xAIResponsesSupportedToolTypes = map[string]struct{}{
	"code_execution":     {},
	"code_interpreter":   {},
	"collections_search": {},
	"file_search":        {},
	"function":           {},
	"mcp":                {},
	"shell":              {},
	"web_search":         {},
	"x_search":           {},
}

func sanitizeXAIResponsesInput(input json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(input)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return input, nil
	}

	var items []json.RawMessage
	if err := common.Unmarshal(trimmed, &items); err != nil {
		return nil, err
	}
	filtered := make([]json.RawMessage, 0, len(items))
	changed := false
	for _, rawItem := range items {
		if common.GetJsonType(rawItem) != "object" {
			filtered = append(filtered, rawItem)
			continue
		}
		var item map[string]json.RawMessage
		if err := common.Unmarshal(rawItem, &item); err != nil {
			return nil, err
		}
		var itemType string
		if rawType, ok := item["type"]; ok {
			if err := common.Unmarshal(rawType, &itemType); err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(itemType) == "additional_tools" {
			changed = true
			continue
		}
		if itemType == "reasoning" {
			if content, ok := item["content"]; ok && bytes.Equal(bytes.TrimSpace(content), []byte("null")) {
				delete(item, "content")
				var err error
				rawItem, err = common.Marshal(item)
				if err != nil {
					return nil, err
				}
				changed = true
			}
		}
		cleaned, itemChanged, err := sanitizeXAIResponsesUnsupportedFields(rawItem)
		if err != nil {
			return nil, err
		}
		changed = changed || itemChanged
		filtered = append(filtered, cleaned)
	}
	if !changed {
		return input, nil
	}
	return common.Marshal(filtered)
}

func sanitizeXAIResponsesTools(tools json.RawMessage, toolChoice json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	trimmed := bytes.TrimSpace(tools)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return tools, toolChoice, nil
	}

	var rawTools []json.RawMessage
	if err := common.Unmarshal(trimmed, &rawTools); err != nil {
		return nil, nil, err
	}
	filtered := make([]json.RawMessage, 0, len(rawTools))
	changed := false
	for _, rawTool := range rawTools {
		var tool struct {
			Type string `json:"type"`
		}
		if err := common.Unmarshal(rawTool, &tool); err != nil {
			return nil, nil, err
		}
		if _, ok := xAIResponsesSupportedToolTypes[strings.TrimSpace(tool.Type)]; !ok {
			changed = true
			continue
		}
		cleaned, toolChanged, err := sanitizeXAIResponsesUnsupportedFields(rawTool)
		if err != nil {
			return nil, nil, err
		}
		changed = changed || toolChanged
		filtered = append(filtered, cleaned)
	}
	if len(filtered) == 0 {
		return nil, nil, nil
	}
	if changed {
		var err error
		tools, err = common.Marshal(filtered)
		if err != nil {
			return nil, nil, err
		}
	}

	if common.GetJsonType(toolChoice) != "object" {
		return tools, toolChoice, nil
	}
	var choice struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := common.Unmarshal(toolChoice, &choice); err != nil {
		return nil, nil, err
	}
	if _, ok := xAIResponsesSupportedToolTypes[strings.TrimSpace(choice.Type)]; !ok {
		return tools, nil, nil
	}
	if choice.Type != "function" {
		return tools, toolChoice, nil
	}
	choiceName := strings.TrimSpace(choice.Name)
	if choiceName == "" {
		choiceName = strings.TrimSpace(choice.Function.Name)
	}
	if choiceName == "" {
		return tools, toolChoice, nil
	}
	for _, rawTool := range filtered {
		var tool struct {
			Type     string `json:"type"`
			Name     string `json:"name"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if err := common.Unmarshal(rawTool, &tool); err != nil {
			return nil, nil, err
		}
		toolName := strings.TrimSpace(tool.Name)
		if toolName == "" {
			toolName = strings.TrimSpace(tool.Function.Name)
		}
		if tool.Type == "function" && toolName == choiceName {
			return tools, toolChoice, nil
		}
	}
	return tools, nil, nil
}

func sanitizeXAIResponsesUnsupportedFields(raw json.RawMessage) (json.RawMessage, bool, error) {
	if !bytes.Contains(raw, []byte(`"external_web_access"`)) {
		return raw, false, nil
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil, false, err
	}
	if !deleteXAIResponsesUnsupportedFields(value) {
		return raw, false, nil
	}
	encoded, err := common.Marshal(value)
	return encoded, true, err
}

func deleteXAIResponsesUnsupportedFields(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		if _, ok := typed["external_web_access"]; ok {
			delete(typed, "external_web_access")
			changed = true
		}
		for _, child := range typed {
			changed = deleteXAIResponsesUnsupportedFields(child) || changed
		}
		return changed
	case []any:
		changed := false
		for _, child := range typed {
			changed = deleteXAIResponsesUnsupportedFields(child) || changed
		}
		return changed
	default:
		return false
	}
}

func xAIModelRejectsReasoning(model string) bool {
	model = strings.TrimSpace(strings.ToLower(model))
	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		model = strings.TrimSpace(model[slash+1:])
	}
	switch model {
	case "grok-composer", "grok-composer-2.5-fast", "composer-2.5":
		return true
	default:
		return false
	}
}
