package dto

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const MaxImageN = 128

type ImageRequest struct {
	Model             string          `json:"model"`
	Prompt            string          `json:"prompt" binding:"required"`
	N                 *uint           `json:"n,omitempty"`
	Size              string          `json:"size,omitempty"`
	AspectRatio       *string         `json:"aspect_ratio,omitempty"`
	Quality           string          `json:"quality,omitempty"`
	ResponseFormat    string          `json:"response_format,omitempty"`
	Style             json.RawMessage `json:"style,omitempty"`
	User              json.RawMessage `json:"user,omitempty"`
	ExtraFields       json.RawMessage `json:"extra_fields,omitempty"`
	Background        json.RawMessage `json:"background,omitempty"`
	Moderation        json.RawMessage `json:"moderation,omitempty"`
	OutputFormat      json.RawMessage `json:"output_format,omitempty"`
	OutputCompression json.RawMessage `json:"output_compression,omitempty"`
	PartialImages     json.RawMessage `json:"partial_images,omitempty"`
	Stream            *bool           `json:"stream,omitempty"`
	Images            json.RawMessage `json:"images,omitempty"`
	Mask              json.RawMessage `json:"mask,omitempty"`
	InputFidelity     json.RawMessage `json:"input_fidelity,omitempty"`
	Watermark         *bool           `json:"watermark,omitempty"`
	// zhipu 4v
	WatermarkEnabled json.RawMessage `json:"watermark_enabled,omitempty"`
	UserId           json.RawMessage `json:"user_id,omitempty"`
	Image            json.RawMessage `json:"image,omitempty"`
	// 用匿名参数接收额外参数
	Extra map[string]json.RawMessage `json:"-"`
}

func (i *ImageRequest) UnmarshalJSON(data []byte) error {
	// 先解析成 map[string]interface{}
	var rawMap map[string]json.RawMessage
	if err := common.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// 用 struct tag 获取所有已定义字段名
	knownFields := GetJSONFieldNames(reflect.TypeOf(*i))

	// 再正常解析已定义字段
	type Alias ImageRequest
	var known Alias
	if err := common.Unmarshal(data, &known); err != nil {
		return err
	}
	*i = ImageRequest(known)

	// 提取多余字段
	i.Extra = make(map[string]json.RawMessage)
	for k, v := range rawMap {
		if _, ok := knownFields[k]; !ok {
			i.Extra[k] = v
		}
	}
	return nil
}

// 序列化时需要重新把字段平铺
func (r ImageRequest) MarshalJSON() ([]byte, error) {
	// 将已定义字段转为 map
	type Alias ImageRequest
	alias := Alias(r)
	base, err := common.Marshal(alias)
	if err != nil {
		return nil, err
	}

	var baseMap map[string]json.RawMessage
	if err := common.Unmarshal(base, &baseMap); err != nil {
		return nil, err
	}

	// 不能合并ExtraFields！！！！！！！！
	// 合并 ExtraFields
	//for k, v := range r.Extra {
	//	if _, exists := baseMap[k]; !exists {
	//		baseMap[k] = v
	//	}
	//}

	return common.Marshal(baseMap)
}

func GetJSONFieldNames(t reflect.Type) map[string]struct{} {
	fields := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 跳过匿名字段（例如 ExtraFields）
		if field.Anonymous {
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}

		// 取逗号前字段名（排除 omitempty 等）
		name := tag
		if commaIdx := indexComma(tag); commaIdx != -1 {
			name = tag[:commaIdx]
		}
		fields[name] = struct{}{}
	}
	return fields
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

func (i *ImageRequest) GetTokenCountMeta() *types.TokenCountMeta {
	var sizeRatio = 1.0
	var qualityRatio = 1.0

	modelName := normalizeImageBillingModelName(i.Model)
	if strings.HasPrefix(modelName, "dall-e") {
		// Size
		if i.Size == "256x256" {
			sizeRatio = 0.4
		} else if i.Size == "512x512" {
			sizeRatio = 0.45
		} else if i.Size == "1024x1024" {
			sizeRatio = 1
		} else if i.Size == "1024x1792" || i.Size == "1792x1024" {
			sizeRatio = 2
		}

		if modelName == "dall-e-3" && i.Quality == "hd" {
			qualityRatio = 2.0
			if i.Size == "1024x1792" || i.Size == "1792x1024" {
				qualityRatio = 1.5
			}
		}
	} else if usesGPTImage2SizeBilling(modelName) {
		sizeRatio = gptImage2SizeRatio(i.Size)
	}

	imageN := uint(1)
	if i.N != nil && *i.N > 0 {
		imageN = *i.N
	}

	// Keep n separate from ImagePriceRatio so size/quality and count remain
	// independent billing dimensions. Fixed-price pre-consume stores this on
	// PriceData, and image settlement reuses or replaces the same "n" ratio.
	return &types.TokenCountMeta{
		CombineText:     i.Prompt,
		MaxTokens:       1584,
		ImagePriceRatio: sizeRatio * qualityRatio,
		BillingRatios:   map[string]float64{"n": float64(imageN)},
	}
}

func normalizeImageBillingModelName(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		model = model[idx+1:]
	}
	return model
}

func usesGPTImage2SizeBilling(model string) bool {
	return model == "gpt-image-2"
}

func gptImage2SizeRatio(size string) float64 {
	size = strings.ToLower(strings.TrimSpace(size))
	size = strings.ReplaceAll(size, "*", "x")
	if size == "" || size == "auto" || size == "1k" || size == "1k_square" {
		return 1
	}
	if size == "2k" || size == "2k_square" {
		return 1.6
	}
	if size == "4k" || size == "4k_square" {
		return 2.4
	}

	width, height, ok := parseImageSize(size)
	if !ok {
		return 1
	}
	switch {
	case width == 2048 && height == 2048:
		return 1.6
	case width == 4096 && height == 4096:
		return 2.4
	case isKnownGPTImage2Size(width, height, "2k"):
		return 1.6
	case isKnownGPTImage2Size(width, height, "4k"):
		return 2.4
	case isKnownGPTImage2Size(width, height, "1k"):
		return 1
	case width*height > 2200*1200:
		return 2.4
	case width*height > 1200*1000:
		return 1.6
	default:
		return 1
	}
}

func parseImageSize(size string) (int, int, bool) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func gptImage2QualityRatio(quality string) float64 {
	return 1
}

func isKnownGPTImage2Size(width, height int, tier string) bool {
	sizes := map[string][][2]int{
		"1k": {
			{1024, 1024}, {1216, 832}, {832, 1216}, {1152, 864}, {864, 1152},
			{1120, 896}, {896, 1120}, {1344, 768}, {768, 1344}, {1536, 640},
		},
		"2k": {
			{1248, 1248}, {1536, 1024}, {1024, 1536}, {1440, 1088}, {1088, 1440},
			{1392, 1120}, {1120, 1392}, {1664, 928}, {928, 1664}, {1904, 816},
		},
		"4k": {
			{2480, 2480}, {3056, 2032}, {2032, 3056}, {2880, 2160}, {2160, 2880},
			{2784, 2224}, {2224, 2784}, {3312, 1872}, {1872, 3312}, {3808, 1632},
		},
	}
	for _, size := range sizes[tier] {
		if width == size[0] && height == size[1] {
			return true
		}
	}
	return false
}

func (i *ImageRequest) IsStream(c *gin.Context) bool {
	return i.Stream != nil && *i.Stream
}

func (i *ImageRequest) SetModelName(modelName string) {
	if modelName != "" {
		i.Model = modelName
	}
}

type ImageResponse struct {
	Data     []ImageData     `json:"data"`
	Created  int64           `json:"created"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
type ImageData struct {
	Url           string `json:"url"`
	B64Json       string `json:"b64_json"`
	RevisedPrompt string `json:"revised_prompt"`
}
