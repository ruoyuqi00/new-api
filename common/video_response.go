package common

// SanitizeVideoTaskResponse removes upstream-only fields before a video task
// response is stored or returned to clients.
func SanitizeVideoTaskResponse(body []byte, publicTaskID string) []byte {
	var response map[string]any
	if err := Unmarshal(body, &response); err != nil {
		return body
	}

	sanitizeTask := func(task map[string]any) {
		delete(task, "billing")
		if publicTaskID == "" {
			return
		}
		if _, ok := task["id"]; ok {
			task["id"] = publicTaskID
		}
		if _, ok := task["task_id"]; ok {
			task["task_id"] = publicTaskID
		}
	}

	sanitizeTask(response)
	if data, ok := response["data"].(map[string]any); ok {
		sanitizeTask(data)
	}
	if providerResponse, ok := response["response"].(map[string]any); ok {
		sanitizeTask(providerResponse)
		delete(providerResponse, "bytesBase64Encoded")
		if video, ok := providerResponse["video"].(string); ok && len(video) > 256 {
			providerResponse["video"] = video[:256] + "..."
		}
		if videos, ok := providerResponse["videos"].([]any); ok {
			for _, video := range videos {
				if item, ok := video.(map[string]any); ok {
					delete(item, "bytesBase64Encoded")
				}
			}
		}
	}

	sanitized, err := Marshal(response)
	if err != nil {
		return body
	}
	return sanitized
}
