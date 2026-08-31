package model

import (
	"strings"
)

type defaultVendorRule struct {
	pattern string
	name    string
}

// Rules are ordered so ambiguous names resolve consistently across processes.
var defaultVendorRules = []defaultVendorRule{
	{pattern: "gpt", name: "OpenAI"},
	{pattern: "sora", name: "OpenAI"},
	{pattern: "dall-e", name: "OpenAI"},
	{pattern: "whisper", name: "OpenAI"},
	{pattern: "o1", name: "OpenAI"},
	{pattern: "o3", name: "OpenAI"},
	{pattern: "claude", name: "Anthropic"},
	{pattern: "gemini", name: "Google"},
	{pattern: "banana", name: "Google"},
	{pattern: "omni", name: "Google"},
	{pattern: "veo", name: "Google"},
	{pattern: "moonshot", name: "Moonshot"},
	{pattern: "kimi", name: "Moonshot"},
	{pattern: "chatglm", name: "智谱"},
	{pattern: "glm-", name: "智谱"},
	{pattern: "qwen", name: "阿里巴巴"},
	{pattern: "deepseek", name: "DeepSeek"},
	{pattern: "abab", name: "MiniMax"},
	{pattern: "minimax", name: "MiniMax"},
	{pattern: "ernie", name: "百度"},
	{pattern: "spark", name: "讯飞"},
	{pattern: "hunyuan", name: "腾讯"},
	{pattern: "command", name: "Cohere"},
	{pattern: "@cf/", name: "Cloudflare"},
	{pattern: "360", name: "360"},
	{pattern: "yi", name: "零一万物"},
	{pattern: "jina", name: "Jina"},
	{pattern: "mistral", name: "Mistral"},
	{pattern: "grok", name: "xAI"},
	{pattern: "llama", name: "Meta"},
	{pattern: "doubao", name: "字节跳动"},
	{pattern: "kling", name: "快手"},
	{pattern: "jimeng", name: "即梦"},
	{pattern: "seedance", name: "即梦"},
	{pattern: "vidu", name: "Vidu"},
}

// 供应商默认图标映射
var defaultVendorIcons = map[string]string{
	"OpenAI":     "OpenAI",
	"Anthropic":  "Claude.Color",
	"Google":     "Gemini.Color",
	"Moonshot":   "Moonshot",
	"智谱":         "Zhipu.Color",
	"阿里巴巴":       "Qwen.Color",
	"DeepSeek":   "DeepSeek.Color",
	"MiniMax":    "Minimax.Color",
	"百度":         "Wenxin.Color",
	"讯飞":         "Spark.Color",
	"腾讯":         "Hunyuan.Color",
	"Cohere":     "Cohere.Color",
	"Cloudflare": "Cloudflare.Color",
	"360":        "Ai360.Color",
	"零一万物":       "Yi.Color",
	"Jina":       "Jina",
	"Mistral":    "Mistral.Color",
	"xAI":        "XAI",
	"Meta":       "Ollama",
	"字节跳动":       "Doubao.Color",
	"快手":         "Kling.Color",
	"即梦":         "Jimeng.Color",
	"Vidu":       "Vidu",
	"微软":         "AzureAI",
	"Microsoft":  "AzureAI",
	"Azure":      "AzureAI",
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}

		// 匹配供应商
		vendorID := 0
		if vendorName := defaultVendorNameForModel(modelName); vendorName != "" {
			vendorID = getOrCreateVendor(vendorName, vendorMap)
		}

		// 创建模型元数据
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  vendorID,
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

func defaultVendorNameForModel(modelName string) string {
	modelLower := strings.ToLower(modelName)
	for _, rule := range defaultVendorRules {
		if strings.Contains(modelLower, rule.pattern) {
			return rule.name
		}
	}
	return ""
}

// 查找或创建供应商
func getOrCreateVendor(vendorName string, vendorMap map[int]*Vendor) int {
	// 查找现有供应商
	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id
		}
	}

	// 创建新供应商
	newVendor := &Vendor{
		Name:   vendorName,
		Status: 1,
		Icon:   getDefaultVendorIcon(vendorName),
	}

	if err := newVendor.Insert(); err != nil {
		return 0
	}

	vendorMap[newVendor.Id] = newVendor
	return newVendor.Id
}

// 获取供应商默认图标
func getDefaultVendorIcon(vendorName string) string {
	if icon, exists := defaultVendorIcons[vendorName]; exists {
		return icon
	}
	return ""
}
