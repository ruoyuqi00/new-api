package service

import (
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

func TestNormalizeYucoreMediaRequestPreservesExplicitOptionalZeroValues(t *testing.T) {
	generateAudio := false
	seed := int64(0)
	selected := yucoreMediaRequestTestModel("seedance-2.0")

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
	selected := yucoreMediaRequestTestModel("seedance-2.0")

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
}

func TestNormalizeYucoreMediaRequestValidatesReferenceModeCombinations(t *testing.T) {
	selected := yucoreMediaRequestTestModel("seedance-2.0")
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
	selected := yucoreMediaRequestTestModel("seedance-2.0")
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

func TestNormalizeYucoreMediaRequestRejectsInvalidReferences(t *testing.T) {
	selected := yucoreMediaRequestTestModel("seedance-2.0")
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

func TestNormalizeYucoreMediaRequestDoesNotMutateCaller(t *testing.T) {
	duration := 5
	referenceDuration := 9000
	generateAudio := false
	seed := int64(0)
	negativePrompt := " avoid blur "
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

	normalized, err := NormalizeYucoreMediaRequest(yucoreMediaRequestTestModel("seedance-2.0"), options)
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
