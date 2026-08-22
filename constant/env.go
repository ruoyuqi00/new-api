package constant

import "strings"

var StreamingTimeout int
var DifyDebug bool
var MaxFileDownloadMB int
var StreamScannerMaxBufferMB int
var StreamUsageDrainEnabled bool
var StreamUsageDrainMaxConcurrency int
var StreamUsageDrainMaxPerChannel int
var StreamUsageDrainTimeoutSeconds int
var StreamUsageDrainMaxBytesMB int
var ForceStreamOption bool
var CountToken bool
var GetMediaToken bool
var GetMediaTokenNotStream bool
var UpdateTask bool
var MaxRequestBodyMB int
var AnonymousRequestBodyLimitKB int
var AzureDefaultAPIVersion string
var NotifyLimitCount int
var NotificationLimitDurationMinute int
var GenerateDefaultToken bool
var ErrorLogEnabled bool
var TaskQueryLimit int
var TaskTimeoutMinutes int

// temporary variable for sora patch, will be removed in future
var TaskPricePatches []string

// TaskPricePatchApplies reports whether task billing must ignore duration and
// other request multipliers. Grok Imagine Video is explicitly duration-based.
func TaskPricePatchApplies(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "grok-imagine-video", "grok-imagine-video-1.5", "grok-imagine-video-1.5-preview":
		return false
	}
	for _, patchedModel := range TaskPricePatches {
		if strings.EqualFold(strings.TrimSpace(patchedModel), strings.TrimSpace(model)) {
			return true
		}
	}
	return false
}

// TrustedRedirectDomains is a list of trusted domains for redirect URL validation.
// Domains support subdomain matching (e.g., "example.com" matches "sub.example.com").
var TrustedRedirectDomains []string
