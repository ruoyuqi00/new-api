package service

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func yucoreMediaRequestTestModel(id string) YucoreMediaCatalogModel {
	return YucoreMediaCatalogModel{
		Id:             id,
		Kind:           YucoreMediaKindVideo,
		Modes:          []string{"text-to-video", "image-to-video"},
		Counts:         []int{1},
		Durations:      []int{4, 5, 6, 8, 10, 12, 15},
		Resolutions:    []string{"480p", "720p"},
		AspectRatios:   []string{"16:9", "9:16", "1:1"},
		ReferenceModes: []string{"media", "frames"},
		SupportsAudio:  true,
		SupportsSeed:   true,
		InputLimits: YucoreMediaCatalogInputLimits{
			MaxReferenceImages: 4,
			MaxReferenceVideos: 3,
			MaxReferenceAudios: 1,
			MaxReferences:      8,
		},
	}
}

func intPointer(value int) *int { return &value }

func yucoreMediaTestDataURL(mimeType string, payload []byte) string {
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(payload)
}

func TestNormalizeYucoreMediaRequestPreservesExplicitOptionalZeroValues(t *testing.T) {
	generateAudio := false
	seed := int64(0)
	selected := yucoreMediaRequestTestModel("request-test-video")

	normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{
		GenerateAudio: &generateAudio,
		Seed:          &seed,
	})
	require.NoError(t, err)
	require.NotNil(t, normalized.GenerateAudio)
	require.NotNil(t, normalized.Seed)
	assert.False(t, *normalized.GenerateAudio)
	assert.Zero(t, *normalized.Seed)

	omitted, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{})
	require.NoError(t, err)
	assert.Nil(t, omitted.GenerateAudio)
	assert.Nil(t, omitted.Seed)
}

func TestNormalizeYucoreMediaRequestValidatesDurationResolutionAndAspectRatio(t *testing.T) {
	selected := yucoreMediaRequestTestModel("request-test-video")

	normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{
		Resolution:  " 720P ",
		AspectRatio: " 9:16 ",
	})
	require.NoError(t, err)
	require.NotNil(t, normalized.Duration)
	assert.Equal(t, 4, *normalized.Duration)
	assert.Equal(t, "720p", normalized.Resolution)
	assert.Equal(t, "9:16", normalized.AspectRatio)

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{Duration: intPointer(7)})
	require.ErrorContains(t, err, "duration")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{Resolution: "4k"})
	require.ErrorContains(t, err, "resolution")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{AspectRatio: "4:3"})
	require.ErrorContains(t, err, "aspect ratio")
}

func TestNormalizeYucoreMediaRequestAllowsResolutionBelowModelMaximum(t *testing.T) {
	for _, test := range []struct {
		name         string
		resolutions  []string
		requested    string
		want         string
		shouldReject string
	}{
		{name: "2k accepts 1k", resolutions: []string{"2k"}, requested: "1k", want: "1k", shouldReject: "4k"},
		{name: "4k accepts 2k", resolutions: []string{"4k"}, requested: "2k", want: "2k", shouldReject: "8k"},
	} {
		t.Run(test.name, func(t *testing.T) {
			selected := YucoreMediaCatalogModel{
				Id:           "image-" + test.name,
				Kind:         YucoreMediaKindImage,
				Modes:        []string{"text-to-image"},
				Counts:       []int{1},
				Resolutions:  test.resolutions,
				AspectRatios: []string{"1:1", "16:9", "9:16"},
			}

			normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{
				Resolution:  test.requested,
				AspectRatio: "16:9",
			})
			require.NoError(t, err)
			assert.Equal(t, test.want, normalized.Resolution)
			assert.Equal(t, "16:9", normalized.AspectRatio)

			_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{Resolution: test.shouldReject})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "resolution")
		})
	}
}

func TestNormalizeYucoreMediaRequestAllowsCustomImageDimensionsWithinCap(t *testing.T) {
	capability := model.YucoreMediaModelCapability{
		Kind:                     YucoreMediaKindImage,
		Resolutions:              []string{"4k"},
		SupportsCustomDimensions: true,
	}
	selected := YucoreMediaCatalogModel{
		Id:           "custom-image-4k",
		Kind:         YucoreMediaKindImage,
		Modes:        []string{"text-to-image"},
		Counts:       []int{1},
		Resolutions:  []string{"1k", "2k", "4k"},
		AspectRatios: []string{"1:1", "16:9", "9:16"},
		capability:   &capability,
	}

	for _, requested := range []string{"650x1024", "1024X650", "4096x1"} {
		normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{
			Resolution:  requested,
			AspectRatio: "16:9",
		})
		require.NoError(t, err, requested)
		assert.Equal(t, strings.ToLower(strings.ReplaceAll(requested, "X", "x")), normalized.Resolution)
		assert.Equal(t, "auto", normalized.AspectRatio)
	}

	for _, requested := range []string{"0x1024", "4097x1"} {
		_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{Resolution: requested})
		require.Error(t, err, requested)
		assert.Contains(t, err.Error(), "dimensions")
	}
	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{Resolution: "bad-size"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolution")
}

func TestNormalizeYucoreMediaRequestRejectsCustomImageDimensionsWhenUnsupported(t *testing.T) {
	selected := YucoreMediaCatalogModel{
		Id:          "fixed-image-1k",
		Kind:        YucoreMediaKindImage,
		Modes:       []string{"text-to-image"},
		Counts:      []int{1},
		Resolutions: []string{"1k"},
	}

	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{Resolution: "650x1024"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "custom image dimensions")
}

func TestNormalizeYucoreMediaRequestKeepsModelMaximumAsDefault(t *testing.T) {
	for _, test := range []struct {
		name        string
		resolutions []string
		want        string
	}{
		{name: "2k", resolutions: []string{"2k"}, want: "2k"},
		{name: "4k", resolutions: []string{"4k"}, want: "4k"},
	} {
		t.Run(test.name, func(t *testing.T) {
			selected := YucoreMediaCatalogModel{
				Id:          "image-default-" + test.name,
				Kind:        YucoreMediaKindImage,
				Modes:       []string{"text-to-image"},
				Counts:      []int{1},
				Resolutions: test.resolutions,
			}

			normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{})
			require.NoError(t, err)
			assert.Equal(t, test.want, normalized.Resolution)
		})
	}
}

func TestNormalizeYucoreMediaRequestPreservesConfiguredDurationOrder(t *testing.T) {
	selected := yucoreMediaRequestTestModel("operator-ordered-video")
	selected.Durations = []int{10, 5}

	normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{})
	require.NoError(t, err)
	require.NotNil(t, normalized.Duration)
	assert.Equal(t, 10, *normalized.Duration)

	normalized, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{Duration: intPointer(5)})
	require.NoError(t, err)
	require.NotNil(t, normalized.Duration)
	assert.Equal(t, 5, *normalized.Duration)
}

func TestNormalizeYucoreMediaRequestDefaultsGrokImagineVideoToFiveSeconds(t *testing.T) {
	selected := YucoreMediaCatalogModel{
		Id:          "grok-imagine-video",
		Kind:        YucoreMediaKindVideo,
		Modes:       []string{"text-to-video", "image-to-video"},
		Counts:      []int{1},
		Durations:   []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		Resolutions: []string{"480p", "720p", "1080p"},
	}

	normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{})
	require.NoError(t, err)
	require.NotNil(t, normalized.Duration)
	assert.Equal(t, 5, *normalized.Duration)
	assert.Equal(t, "480p", normalized.Resolution)
}

func TestNormalizeYucoreMediaRequestRejectsUnsupportedOptionalParameters(t *testing.T) {
	generateAudio := false
	seed := int64(0)
	negativePrompt := "avoid blur"
	gemini := yucoreMediaRequestTestModel("gemini-omni-flash")
	gemini.SupportsAudio = false

	_, err := NormalizeYucoreMediaRequest(gemini, YucoreMediaRequestOptions{GenerateAudio: &generateAudio})
	require.ErrorContains(t, err, "generate audio")

	veo := yucoreMediaRequestTestModel("veo-3.1")
	veo.SupportsSeed = false
	_, err = NormalizeYucoreMediaRequest(veo, YucoreMediaRequestOptions{Seed: &seed})
	require.ErrorContains(t, err, "seed")

	_, err = NormalizeYucoreMediaRequest(veo, YucoreMediaRequestOptions{NegativePrompt: &negativePrompt})
	require.ErrorContains(t, err, "negative prompt")

	emptyNegativePrompt := "  "
	normalized, err := NormalizeYucoreMediaRequest(veo, YucoreMediaRequestOptions{NegativePrompt: &emptyNegativePrompt})
	require.NoError(t, err)
	assert.Nil(t, normalized.NegativePrompt)
}

func TestNormalizeYucoreMediaRequestValidatesReferenceModeCombinations(t *testing.T) {
	selected := yucoreMediaRequestTestModel("request-test-video")
	first := model.YucoreMediaReferenceInput{Role: "first_frame", URL: "https://cdn.example/first.png"}
	last := model.YucoreMediaReferenceInput{Role: "last_frame", URL: "https://cdn.example/last.png"}
	image := model.YucoreMediaReferenceInput{Role: "image", URL: "https://cdn.example/ref.png"}

	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{first}})
	require.ErrorContains(t, err, "first_frame and last_frame")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{last}})
	require.ErrorContains(t, err, "first_frame and last_frame")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{first, last, image}})
	require.ErrorContains(t, err, "frame references")

	normalized, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{first, last}})
	require.NoError(t, err)
	assert.Equal(t, "frames", normalized.ReferenceMode)

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{ReferenceMode: "text", References: []model.YucoreMediaReferenceInput{image}})
	require.ErrorContains(t, err, "text reference mode")
}

func TestNormalizeYucoreMediaRequestRequiresPrimaryImageForMixedFamilyReferences(t *testing.T) {
	selected := yucoreMediaRequestTestModel("kling-3.0-omni")
	selected.RequirePrimaryImageForMedia = true
	selected.Durations = []int{5, 10}
	selected.Resolutions = []string{"720p", "1080p"}
	selected.InputLimits = YucoreMediaCatalogInputLimits{
		MaxReferenceImages:          4,
		MaxReferenceVideos:          1,
		MaxReferenceAudios:          1,
		MaxReferences:               6,
		MaxReferenceVideoDurationMS: 10000,
		MaxReferenceAudioDurationMS: 30000,
	}

	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{{Role: "video", URL: "https://cdn.example/ref.mp4"}}})
	require.ErrorContains(t, err, "primary image")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{{Role: "audio", URL: "https://cdn.example/ref.mp3"}}})
	require.ErrorContains(t, err, "primary image")

	standalone := yucoreMediaRequestTestModel("omni-v2v")
	standalone.Durations = []int{10}
	standalone.Resolutions = []string{"720p"}
	standalone.ReferenceModes = []string{"media"}
	standalone.InputLimits = YucoreMediaCatalogInputLimits{MaxReferenceVideos: 1, MaxReferences: 1}
	_, err = NormalizeYucoreMediaRequest(standalone, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{{Role: "video", URL: "https://cdn.example/ref.mp4"}}})
	require.NoError(t, err)
}

func TestNormalizeYucoreMediaRequestValidatesReferenceLimitsAndDurations(t *testing.T) {
	selected := yucoreMediaRequestTestModel("request-test-video")
	images := make([]model.YucoreMediaReferenceInput, 5)
	for index := range images {
		images[index] = model.YucoreMediaReferenceInput{Role: "image", URL: "https://cdn.example/image.png"}
	}
	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: images})
	require.ErrorContains(t, err, "reference image")

	references := append([]model.YucoreMediaReferenceInput(nil), images[:4]...)
	for index := 0; index < 3; index++ {
		references = append(references, model.YucoreMediaReferenceInput{Role: "video", URL: "https://cdn.example/video.mp4"})
	}
	references = append(references,
		model.YucoreMediaReferenceInput{Role: "audio", URL: "https://cdn.example/audio.mp3"},
		model.YucoreMediaReferenceInput{Role: "image", URL: "https://cdn.example/extra.png"},
	)
	selected.InputLimits.MaxReferenceImages = 5
	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: references})
	require.ErrorContains(t, err, "total")

	kling := yucoreMediaRequestTestModel("kling-3.0-omni")
	kling.Durations = []int{5, 10}
	kling.InputLimits = YucoreMediaCatalogInputLimits{MaxReferenceImages: 4, MaxReferenceVideos: 1, MaxReferenceAudios: 1, MaxReferences: 6, MaxReferenceVideoDurationMS: 10000, MaxReferenceAudioDurationMS: 30000}
	_, err = NormalizeYucoreMediaRequest(kling, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{
		{Role: "image", URL: "https://cdn.example/image.png"},
		{Role: "video", URL: "https://cdn.example/video.mp4", DurationMS: intPointer(10001)},
	}})
	require.ErrorContains(t, err, "video duration")

	_, err = NormalizeYucoreMediaRequest(kling, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{
		{Role: "image", URL: "https://cdn.example/image.png"},
		{Role: "audio", URL: "https://cdn.example/audio.mp3", DurationMS: intPointer(30001)},
	}})
	require.ErrorContains(t, err, "audio duration")
}

func TestNormalizeYucoreMediaRequestEnforcesObservedReferenceConstraints(t *testing.T) {
	selected := yucoreMediaRequestTestModel("conditional-video")
	selected.InputLimits.MinReferenceVideoDurationMS = 3000
	selected.InputLimits.MaxReferenceVideoDurationMS = 10000
	selected.InputLimits.MaxTotalReferenceVideoDurationMS = 10000
	selected.InputLimits.MaxReferenceAudioDurationMS = 15000
	selected.InputLimits.MaxTotalReferenceAudioDurationMS = 15000
	selected.InputLimits.MaxImagesWithVideo = 1
	selected.RequiredReferenceKinds = []string{"video"}

	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{})
	require.ErrorContains(t, err, "requires a video reference")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{
		{Role: "image", URL: "https://cdn.example/a.png"},
		{Role: "image", URL: "https://cdn.example/b.png"},
		{Role: "video", URL: "https://cdn.example/a.mp4", DurationMS: intPointer(5000)},
	}})
	require.ErrorContains(t, err, "at most 1 reference image")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{
		{Role: "video", URL: "https://cdn.example/a.mp4", DurationMS: intPointer(2000)},
	}})
	require.ErrorContains(t, err, "at least 3000 ms")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{
		{Role: "video", URL: "https://cdn.example/a.mp4", DurationMS: intPointer(6000)},
		{Role: "video", URL: "https://cdn.example/b.mp4", DurationMS: intPointer(5000)},
	}})
	require.ErrorContains(t, err, "total reference video duration exceeds 10000 ms")

	_, err = NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{
		{Role: "video", URL: "https://cdn.example/a.mp4", DurationMS: intPointer(5000)},
		{Role: "audio", URL: "https://cdn.example/a.mp3", DurationMS: intPointer(8000)},
		{Role: "audio", URL: "https://cdn.example/b.mp3", DurationMS: intPointer(8000)},
	}})
	require.ErrorContains(t, err, "total reference audio duration exceeds 15000 ms")
}

func TestNormalizeYucoreMediaRequestRejectsGeneratedAudioWithFrames(t *testing.T) {
	selected := yucoreMediaRequestTestModel("frame-video")
	selected.DisallowGeneratedAudioWithFrames = true
	generateAudio := true
	_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{
		GenerateAudio: &generateAudio,
		References: []model.YucoreMediaReferenceInput{
			{Role: "first_frame", URL: "https://cdn.example/first.png"},
			{Role: "last_frame", URL: "https://cdn.example/last.png"},
		},
	})
	require.ErrorContains(t, err, "generated audio with frame references")
}

func TestNormalizeYucoreMediaRequestRejectsInvalidReferences(t *testing.T) {
	selected := yucoreMediaRequestTestModel("request-test-video")
	tests := []struct {
		name    string
		options YucoreMediaRequestOptions
		error   string
	}{
		{name: "unknown role", options: YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{{Role: "document", URL: "https://cdn.example/ref"}}}, error: "role"},
		{name: "blank url", options: YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{{Role: "image", URL: "  "}}}, error: "URL"},
		{name: "unknown mode", options: YucoreMediaRequestOptions{ReferenceMode: "gallery"}, error: "reference mode"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeYucoreMediaRequest(selected, test.options)
			require.ErrorContains(t, err, test.error)
		})
	}
}

func TestNormalizeYucoreMediaRequestAcceptsSafeReferenceValues(t *testing.T) {
	selected := yucoreMediaRequestTestModel("request-test-video")
	pngDataURL := yucoreMediaTestDataURL("image/png", append([]byte("\x89PNG\r\n\x1a\n"), []byte("payload")...))
	jpegDataURL := yucoreMediaTestDataURL("image/jpeg", []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'})
	jpgAliasDataURL := yucoreMediaTestDataURL("image/jpg", []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'})
	webpDataURL := yucoreMediaTestDataURL("image/webp", []byte{'R', 'I', 'F', 'F', 0x08, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '})
	gifDataURL := yucoreMediaTestDataURL("image/gif", []byte("GIF89a\x01\x00\x01\x00"))
	tests := []struct {
		name       string
		references []model.YucoreMediaReferenceInput
	}{
		{name: "http URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: "http://cdn.example.com/reference.png"}}},
		{name: "https URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: "https://cdn.example.com/reference.png?size=large#preview"}}},
		{name: "public IPv4 URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: "https://8.8.8.8/reference.png"}}},
		{name: "public IPv6 URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: "https://[2606:4700:4700::1111]/reference.png"}}},
		{name: "unsigned cached upload", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: "/api/yucore/media/uploads/42/ref_1234567890_AbCd123456.png"}}},
		{name: "signed cached upload", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: "/api/yucore/media/uploads/42/ref_1234567890_AbCd123456.png?sig=abc123"}}},
		{name: "legacy ref ID", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: "ref_1234567890_AbCd123456"}}},
		{name: "legacy asset ID", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: "asset_1"}}},
		{name: "PNG data URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: pngDataURL}}},
		{name: "JPEG data URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: jpegDataURL}}},
		{name: "JPG alias data URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: jpgAliasDataURL}}},
		{name: "WebP data URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: webpDataURL}}},
		{name: "GIF data URL", references: []model.YucoreMediaReferenceInput{{Role: "image", URL: gifDataURL}}},
		{name: "frame data URLs", references: []model.YucoreMediaReferenceInput{
			{Role: "first_frame", URL: pngDataURL},
			{Role: "last_frame", URL: jpegDataURL},
		}},
		{name: "mixed remote media", references: []model.YucoreMediaReferenceInput{
			{Role: "image", URL: "https://cdn.example.com/primary.png"},
			{Role: "video", URL: "https://cdn.example.com/reference.mp4"},
			{Role: "audio", URL: "https://cdn.example.com/reference.mp3"},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: test.references})
			require.NoError(t, err)
		})
	}
}

func TestNormalizeYucoreMediaRequestRejectsUnsafeReferenceValues(t *testing.T) {
	selected := yucoreMediaRequestTestModel("request-test-video")
	pngPayload := append([]byte("\x89PNG\r\n\x1a\n"), []byte("payload")...)
	jpegPayload := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	tests := []struct {
		name      string
		role      string
		value     string
		errorText string
	}{
		{name: "newline", role: "image", value: "https://cdn.example.com/ref.png\nheader", errorText: "control"},
		{name: "null byte", role: "image", value: "https://cdn.example.com/ref\x00.png", errorText: "control"},
		{name: "file scheme", role: "image", value: "file:///etc/passwd", errorText: "reference value"},
		{name: "javascript scheme", role: "image", value: "javascript:alert(1)", errorText: "reference value"},
		{name: "ftp scheme", role: "image", value: "ftp://cdn.example.com/ref.png", errorText: "reference value"},
		{name: "userinfo", role: "image", value: "https://user:pass@cdn.example.com/ref.png", errorText: "userinfo"},
		{name: "missing host", role: "image", value: "https:///ref.png", errorText: "host"},
		{name: "invalid host", role: "image", value: "https://bad_host/ref.png", errorText: "host"},
		{name: "localhost", role: "image", value: "http://localhost/ref.png", errorText: "public host"},
		{name: "localhost trailing dot", role: "image", value: "http://LOCALHOST./ref.png", errorText: "public host"},
		{name: "localhost subdomain", role: "image", value: "http://media.localhost/ref.png", errorText: "public host"},
		{name: "loopback IPv4", role: "image", value: "http://127.0.0.1/ref.png", errorText: "public host"},
		{name: "integer IPv4", role: "image", value: "http://2130706433/ref.png", errorText: "public host"},
		{name: "hex IPv4", role: "image", value: "http://0x7f000001/ref.png", errorText: "public host"},
		{name: "short IPv4", role: "image", value: "http://127.1/ref.png", errorText: "public host"},
		{name: "octal IPv4", role: "image", value: "http://0177.0.0.1/ref.png", errorText: "public host"},
		{name: "unspecified IPv4", role: "image", value: "http://0.0.0.0/ref.png", errorText: "public host"},
		{name: "private 10 range", role: "image", value: "http://10.0.0.1/ref.png", errorText: "public host"},
		{name: "private 172 range", role: "image", value: "http://172.16.0.1/ref.png", errorText: "public host"},
		{name: "private 192 range", role: "image", value: "http://192.168.0.1/ref.png", errorText: "public host"},
		{name: "link local metadata", role: "image", value: "http://169.254.169.254/latest/meta-data", errorText: "public host"},
		{name: "multicast IPv4", role: "image", value: "http://224.0.0.1/ref.png", errorText: "public host"},
		{name: "loopback IPv6", role: "image", value: "http://[::1]/ref.png", errorText: "public host"},
		{name: "ULA IPv6", role: "image", value: "http://[fc00::1]/ref.png", errorText: "public host"},
		{name: "link local IPv6", role: "image", value: "http://[fe80::1]/ref.png", errorText: "public host"},
		{name: "NAT64 well known", role: "image", value: "http://[64:ff9b::a9fe:a9fe]/ref.png", errorText: "public host"},
		{name: "NAT64 local use", role: "image", value: "http://[64:ff9b:1::a9fe:a9fe]/ref.png", errorText: "public host"},
		{name: "6to4 link local", role: "image", value: "http://[2002:a9fe:a9fe::]/ref.png", errorText: "public host"},
		{name: "deprecated site local", role: "image", value: "http://[fec0::1]/ref.png", errorText: "public host"},
		{name: "IPv4 compatible", role: "image", value: "http://[::a9fe:a9fe]/ref.png", errorText: "public host"},
		{name: "IPv4 translated", role: "image", value: "http://[::ffff:0:a9fe:a9fe]/ref.png", errorText: "public host"},
		{name: "IPv4 mapped link local", role: "image", value: "http://[::ffff:169.254.169.254]/ref.png", errorText: "public host"},
		{name: "Teredo transition", role: "image", value: "http://[2001:0:4136:e378:8000:63bf:3fff:fdd2]/ref.png", errorText: "public host"},
		{name: "encoded control", role: "image", value: "https://cdn.example.com/ref%0A.png", errorText: "control"},
		{name: "literal space", role: "image", value: "https://cdn.example.com/ref image.png", errorText: "reference value"},
		{name: "backslash path", role: "image", value: "https://cdn.example.com/ref\\image.png", errorText: "reference value"},
		{name: "text data URL", role: "image", value: "data:text/plain;base64,aGVsbG8=", errorText: "supported raster"},
		{name: "SVG data URL", role: "image", value: yucoreMediaTestDataURL("image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)), errorText: "supported raster"},
		{name: "arbitrary image data URL", role: "image", value: yucoreMediaTestDataURL("image/bmp", []byte("BMpayload")), errorText: "supported raster"},
		{name: "malformed base64", role: "image", value: "data:image/png;base64,@@@", errorText: "base64"},
		{name: "percent encoded payload", role: "image", value: "data:image/png,%89PNG", errorText: "base64"},
		{name: "empty payload", role: "image", value: "data:image/png;base64,", errorText: "nonempty"},
		{name: "declared content mismatch", role: "image", value: yucoreMediaTestDataURL("image/png", jpegPayload), errorText: "does not match"},
		{name: "invalid detected content", role: "image", value: yucoreMediaTestDataURL("image/png", []byte("not an image")), errorText: "does not match"},
		{name: "oversized decoded image", role: "image", value: yucoreMediaTestDataURL("image/png", append(pngPayload, []byte(strings.Repeat("A", 512*1024))...)), errorText: "too large"},
		{name: "video data URL", role: "video", value: "data:image/png;base64,aGVsbG8=", errorText: "data URL"},
		{name: "audio data URL", role: "audio", value: "data:image/png;base64,aGVsbG8=", errorText: "data URL"},
		{name: "oversized data URL", role: "image", value: "data:image/png;base64," + strings.Repeat("A", 512*1024), errorText: "too large"},
		{name: "local traversal", role: "image", value: "/api/yucore/media/uploads/42/../secret.png", errorText: "upload path"},
		{name: "encoded local traversal", role: "image", value: "/api/yucore/media/uploads/42/%2e%2e/secret.png", errorText: "upload path"},
		{name: "wrong local route", role: "image", value: "/api/yucore/media/tasks/42/ref.png", errorText: "reference value"},
		{name: "unknown opaque ID", role: "image", value: "reference-123", errorText: "reference value"},
		{name: "malformed ref ID", role: "image", value: "ref_x", errorText: "reference value"},
		{name: "malformed asset ID", role: "image", value: "asset_bad", errorText: "reference value"},
		{name: "oversized opaque ID", role: "image", value: "ref_" + strings.Repeat("a", 200), errorText: "reference value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeYucoreMediaRequest(selected, YucoreMediaRequestOptions{References: []model.YucoreMediaReferenceInput{{Role: test.role, URL: test.value}}})
			require.ErrorContains(t, err, test.errorText)
		})
	}
}

func TestNormalizeYucoreMediaRequestDoesNotMutateCaller(t *testing.T) {
	duration := 5
	referenceDuration := 9000
	generateAudio := false
	seed := int64(0)
	negativePrompt := " avoid blur "
	selected := yucoreMediaRequestTestModel("request-copy-test")
	selected.capability = &model.YucoreMediaModelCapability{AllowedParameters: []string{"negative_prompt"}}
	references := []model.YucoreMediaReferenceInput{{Role: " IMAGE ", URL: " https://cdn.example/ref.png ", MimeType: " image/png ", DurationMS: &referenceDuration}}
	options := YucoreMediaRequestOptions{
		Duration:       &duration,
		Resolution:     " 720P ",
		AspectRatio:    " 16:9 ",
		GenerateAudio:  &generateAudio,
		Seed:           &seed,
		NegativePrompt: &negativePrompt,
		References:     references,
	}

	normalized, err := NormalizeYucoreMediaRequest(selected, options)
	require.NoError(t, err)
	assert.Equal(t, " IMAGE ", references[0].Role)
	assert.Equal(t, " https://cdn.example/ref.png ", references[0].URL)
	assert.Equal(t, 9000, *references[0].DurationMS)
	assert.NotSame(t, references[0].DurationMS, normalized.References[0].DurationMS)
	assert.NotSame(t, options.Duration, normalized.Duration)
	assert.NotSame(t, options.GenerateAudio, normalized.GenerateAudio)
	assert.NotSame(t, options.Seed, normalized.Seed)
	assert.NotSame(t, options.NegativePrompt, normalized.NegativePrompt)
	assert.Equal(t, "image", normalized.References[0].Role)
	assert.Equal(t, "https://cdn.example/ref.png", normalized.References[0].URL)
}
