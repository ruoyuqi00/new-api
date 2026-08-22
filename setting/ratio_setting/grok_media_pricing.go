package ratio_setting

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	GrokImageGenerationPrice = 0.02619
	GrokVideoBasePrice       = 0.0414
)

type GrokImageOperation string

const (
	GrokImageOperationGenerate GrokImageOperation = "generate"
)

func GrokImagePrice(operation GrokImageOperation) (float64, error) {
	switch operation {
	case GrokImageOperationGenerate:
		return GrokImageGenerationPrice, nil
	default:
		return 0, fmt.Errorf("unsupported Grok image operation %q", operation)
	}
}

func GrokVideoSecondPrice(resolution string) (float64, error) {
	switch normalizeGrokVideoResolution(resolution) {
	case "480p":
		return 0.0414, nil
	case "720p":
		return 0.0594, nil
	case "1080p":
		return 0.0774, nil
	default:
		return 0, fmt.Errorf("unsupported Grok video resolution %q", resolution)
	}
}

func GrokVideoBillingRatio(resolution string, seconds int) (float64, error) {
	if seconds < 1 || seconds > 15 {
		return 0, fmt.Errorf("unsupported Grok video duration %d seconds", seconds)
	}
	secondPrice, err := GrokVideoSecondPrice(resolution)
	if err != nil {
		return 0, err
	}
	return secondPrice * float64(seconds) / GrokVideoBasePrice, nil
}

func IsGrokImageGenerationModel(model string) bool {
	return model == "grok-imagine-image" || model == "grok-imagine-image-quality"
}

func IsGrokImageEditModel(model string) bool {
	return model == "grok-imagine-edit"
}

func IsGrokImagineVideoModel(model string) bool {
	switch model {
	case "grok-imagine-video", "grok-imagine-video-1.5", "grok-imagine-video-1.5-preview":
		return true
	default:
		return false
	}
}

func normalizeGrokVideoResolution(resolution string) string {
	value := strings.ToLower(strings.TrimSpace(resolution))
	if parts := strings.Split(value, "x"); len(parts) == 2 {
		if _, widthErr := strconv.Atoi(parts[0]); widthErr == nil {
			if _, heightErr := strconv.Atoi(parts[1]); heightErr == nil {
				for _, candidate := range []string{"1080", "720", "480"} {
					if parts[0] == candidate || parts[1] == candidate {
						return candidate + "p"
					}
				}
			}
		}
	}
	if !strings.HasSuffix(value, "p") {
		value += "p"
	}
	return value
}
