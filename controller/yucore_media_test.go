package controller

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func yucoreMediaControllerTestModel(id string) service.YucoreMediaCatalogModel {
	return service.YucoreMediaCatalogModel{
		Id:             id,
		Kind:           service.YucoreMediaKindVideo,
		Modes:          []string{"text-to-video", "image-to-video"},
		Counts:         []int{1},
		Durations:      []int{4, 5, 6, 8, 10, 12, 15},
		Resolutions:    []string{"480p", "720p"},
		AspectRatios:   []string{"16:9", "9:16", "1:1"},
		ReferenceModes: []string{"media", "frames"},
		SupportsAudio:  true,
		SupportsSeed:   true,
		InputLimits: service.YucoreMediaCatalogInputLimits{
			MaxReferenceImages: 4,
			MaxReferenceVideos: 3,
			MaxReferenceAudios: 1,
			MaxReferences:      8,
		},
	}
}

func controllerIntPointer(value int) *int       { return &value }
func controllerInt64Pointer(value int64) *int64 { return &value }
func controllerBoolPointer(value bool) *bool    { return &value }
func controllerStringPointer(value string) *string {
	return &value
}

func mustYucoreMediaJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}

func TestBuildYucoreMediaTaskPreservesBillingGroup(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Group:   "multimodal",
		Kind:    "image",
		ModelId: "gpt-image-1",
		Prompt:  "test prompt",
	}, 42)
	require.NoError(t, err)
	assert.Equal(t, "multimodal", task.BillingGroup)

	response := buildYucoreMediaTaskResponse(&model.YucoreMediaTask{
		BillingGroup: "multimodal",
	})
	assert.Equal(t, "multimodal", response.Group)
}

func TestBuildYucoreMediaTaskPreservesOptionalZeroValues(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:          "video",
		ModelId:       "seedance-2.0",
		Prompt:        "test prompt",
		GenerateAudio: controllerBoolPointer(false),
		Seed:          controllerInt64Pointer(0),
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0")))

	var metadata map[string]json.RawMessage
	require.NoError(t, common.Unmarshal([]byte(task.Metadata), &metadata))
	assert.JSONEq(t, "false", string(metadata["generate_audio"]))
	assert.JSONEq(t, "0", string(metadata["seed"]))

	omitted, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:    "video",
		ModelId: "seedance-2.0",
		Prompt:  "test prompt",
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(omitted, yucoreMediaControllerTestModel("seedance-2.0")))
	metadata = nil
	require.NoError(t, common.Unmarshal([]byte(omitted.Metadata), &metadata))
	_, hasAudio := metadata["generate_audio"]
	_, hasSeed := metadata["seed"]
	assert.False(t, hasAudio)
	assert.False(t, hasSeed)
}

func TestYucoreMediaCanonicalLegacyInputs(t *testing.T) {
	rawInputs := mustYucoreMediaJSON(t, []any{
		" https://cdn.example/string.png ",
		map[string]any{"url": "url", "source_url": "source", "cachedUrl": "cached", "dataUrl": " data "},
		map[string]any{"data_url": "data_snake", "cached_url": "cached_snake"},
		map[string]any{"cached_url": "cached_snake", "sourceUrl": "source_camel"},
		map[string]any{"source_url": "source_snake", "url": "url"},
		map[string]any{"url": "url", "path": "path"},
		map[string]any{"path": "path", "id": "id"},
		map[string]any{"id": "id"},
		map[string]any{"kind": " VIDEO ", "url": "video", "mimeType": "video/mp4", "durationMs": 1200},
		map[string]any{"role": "audio", "kind": "video", "url": "audio", "mime_type": "audio/mpeg", "duration_ms": 2300},
	})
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:    "video",
		ModelId: "seedance-2.0",
		Prompt:  "test prompt",
		Inputs:  rawInputs,
	}, 42)
	require.NoError(t, err)

	var references []model.YucoreMediaReferenceInput
	require.NoError(t, common.Unmarshal([]byte(task.Inputs), &references))
	require.Len(t, references, 10)
	assert.Equal(t, []string{"https://cdn.example/string.png", "data", "data_snake", "cached_snake", "source_snake", "url", "path", "id", "video", "audio"}, []string{
		references[0].URL, references[1].URL, references[2].URL, references[3].URL, references[4].URL,
		references[5].URL, references[6].URL, references[7].URL, references[8].URL, references[9].URL,
	})
	assert.Equal(t, "image", references[0].Role)
	assert.Equal(t, "image", references[1].Role)
	assert.Equal(t, "video", strings.ToLower(strings.TrimSpace(references[8].Role)))
	assert.Equal(t, "audio", references[9].Role)
	assert.Equal(t, "video/mp4", references[8].MimeType)
	require.NotNil(t, references[8].DurationMS)
	assert.Equal(t, 1200, *references[8].DurationMS)
	assert.Equal(t, "audio/mpeg", references[9].MimeType)
	require.NotNil(t, references[9].DurationMS)
	assert.Equal(t, 2300, *references[9].DurationMS)
}

func TestYucoreMediaCanonicalTopLevelFieldsOverrideLegacyMetadata(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:           "video",
		ModelId:        "seedance-2.0",
		Prompt:         "test prompt",
		Duration:       controllerIntPointer(5),
		Resolution:     controllerStringPointer("720P"),
		GenerateAudio:  controllerBoolPointer(false),
		Seed:           controllerInt64Pointer(0),
		ReferenceMode:  controllerStringPointer("text"),
		NegativePrompt: controllerStringPointer("top level"),
		Metadata: mustYucoreMediaJSON(t, map[string]any{
			"durationSeconds": 10,
			"resolution":      "480p",
			"generateAudio":   true,
			"seed":            99,
			"referenceMode":   "media",
			"negativePrompt":  "metadata",
			"ui_state":        "keep",
		}),
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0")))

	var metadata map[string]json.RawMessage
	require.NoError(t, common.Unmarshal([]byte(task.Metadata), &metadata))
	assert.JSONEq(t, "5", string(metadata["duration"]))
	assert.JSONEq(t, `"720p"`, string(metadata["resolution"]))
	assert.JSONEq(t, "false", string(metadata["generate_audio"]))
	assert.JSONEq(t, "0", string(metadata["seed"]))
	assert.JSONEq(t, `"text"`, string(metadata["reference_mode"]))
	assert.JSONEq(t, `"top level"`, string(metadata["negative_prompt"]))
	assert.JSONEq(t, `"keep"`, string(metadata["ui_state"]))
	assert.Equal(t, "720p", task.Size)
	assert.Equal(t, "top level", task.NegativePrompt)
}

func TestYucoreMediaCanonicalReadsLegacyMetadataWhenTopLevelIsAbsent(t *testing.T) {
	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:    "video",
		ModelId: "seedance-2.0",
		Prompt:  "test prompt",
		Metadata: mustYucoreMediaJSON(t, map[string]any{
			"duration_seconds": 8,
			"resolution":       "720P",
			"generateAudio":    false,
			"seed":             0,
			"referenceMode":    "text",
			"negativePrompt":   "metadata value",
		}),
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0")))

	var metadata map[string]json.RawMessage
	require.NoError(t, common.Unmarshal([]byte(task.Metadata), &metadata))
	assert.JSONEq(t, "8", string(metadata["duration"]))
	assert.JSONEq(t, `"720p"`, string(metadata["resolution"]))
	assert.JSONEq(t, "false", string(metadata["generate_audio"]))
	assert.JSONEq(t, "0", string(metadata["seed"]))
	assert.Equal(t, "metadata value", task.NegativePrompt)
}

func TestYucoreMediaCanonicalTreatsFrontendEmptyNegativePromptAsOmitted(t *testing.T) {
	var req yucoreMediaTaskRequest
	require.NoError(t, common.Unmarshal([]byte(`{
		"kind":"video",
		"model_id":"veo-3.1",
		"prompt":"test prompt",
		"negative_prompt":"",
		"inputs":[],
		"metadata":{}
	}`), &req))
	task, err := buildYucoreMediaTaskFromRequest(req, 42)
	require.NoError(t, err)
	selected := yucoreMediaControllerTestModel("veo-3.1")
	selected.SupportsSeed = false
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(task, selected))
	assert.Empty(t, task.NegativePrompt)

	var metadata map[string]json.RawMessage
	require.NoError(t, common.Unmarshal([]byte(task.Metadata), &metadata))
	_, present := metadata["negative_prompt"]
	assert.False(t, present)
}

func TestYucoreMediaCanonicalEnforcesSelectedPromptLimitByRunes(t *testing.T) {
	selected := yucoreMediaControllerTestModel("operator-prompt-limit")
	selected.InputLimits.MaxPromptChars = 4

	atLimit, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:   "video",
		Prompt: "图像测试",
	}, 42)
	require.NoError(t, err)
	require.NoError(t, normalizeYucoreMediaTaskWithSelection(atLimit, selected))

	overLimit, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:   "video",
		Prompt: "图像测试超",
	}, 42)
	require.NoError(t, err)
	err = normalizeYucoreMediaTaskWithSelection(overLimit, selected)
	require.ErrorContains(t, err, "prompt is too long")
}

func TestYucoreMediaCanonicalRejectsFractionalReferenceDuration(t *testing.T) {
	_, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{
		Kind:    "video",
		ModelId: "seedance-2.0",
		Prompt:  "test prompt",
		Inputs:  json.RawMessage(`[{"url":"video","kind":"video","duration_ms":1.5}]`),
	}, 42)
	require.ErrorContains(t, err, "duration_ms")
}

func TestYucoreMediaCanonicalPreservesExistingRequestContracts(t *testing.T) {
	_, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{Prompt: "  "}, 42)
	require.ErrorContains(t, err, "prompt is required")

	_, err = buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{Prompt: "ok", SessionId: strings.Repeat("x", maxYucoreMediaSessionIdRunes+1)}, 42)
	require.ErrorContains(t, err, "session id is too long")

	task, err := buildYucoreMediaTaskFromRequest(yucoreMediaTaskRequest{Kind: "video", Prompt: "ok", Count: 2}, 42)
	require.NoError(t, err)
	err = normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0"))
	require.ErrorContains(t, err, "count")

	task.Count = 1
	task.Mode = "text-to-image"
	err = normalizeYucoreMediaTaskWithSelection(task, yucoreMediaControllerTestModel("seedance-2.0"))
	require.ErrorContains(t, err, "mode")
}

func yucoreMediaUploadTestContent() map[string][]byte {
	mpegFrames := yucoreMediaTestMPEGFrames(2)
	id3Audio := yucoreMediaTestID3Audio(4, mpegFrames)
	return map[string][]byte{
		"image/png":       append([]byte("\x89PNG\r\n\x1a\n"), []byte("payload")...),
		"image/jpeg":      {0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
		"image/webp":      append([]byte("RIFF\x08\x00\x00\x00WEBPVP8 "), []byte("payload")...),
		"image/gif":       []byte("GIF89a\x01\x00\x01\x00"),
		"video/mp4":       append([]byte("\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isommp41"), []byte("payload")...),
		"video/quicktime": append([]byte("\x00\x00\x00\x14ftypqt  \x00\x00\x00\x00qt  "), []byte("payload")...),
		"audio/mpeg":      id3Audio,
		"audio/wav":       append([]byte("RIFF\x08\x00\x00\x00WAVE"), []byte("payload")...),
	}
}

func yucoreMediaTestMPEGFrames(count int) []byte {
	return yucoreMediaTestMPEGFramesWithHeader(count, 417, []byte{0xff, 0xfb, 0x90, 0x64})
}

func yucoreMediaTestMPEGFramesWithHeader(count int, frameLength int, header []byte) []byte {
	frames := make([]byte, frameLength*count)
	for index := 0; index < count; index++ {
		copy(frames[index*frameLength:], header)
	}
	return frames
}

func yucoreMediaTestID3Audio(tagSize int, frames []byte) []byte {
	header := []byte("ID3\x04\x00\x00\x00\x00\x00\x00")
	header[6] = byte(tagSize>>21) & 0x7f
	header[7] = byte(tagSize>>14) & 0x7f
	header[8] = byte(tagSize>>7) & 0x7f
	header[9] = byte(tagSize) & 0x7f
	content := append(header, make([]byte, tagSize)...)
	return append(content, frames...)
}

func yucoreMediaTestFTYP(majorBrand string, compatibleBrands ...string) []byte {
	box := make([]byte, 16+4*len(compatibleBrands))
	binary.BigEndian.PutUint32(box[:4], uint32(len(box)))
	copy(box[4:8], "ftyp")
	copy(box[8:12], majorBrand)
	for index, brand := range compatibleBrands {
		copy(box[16+index*4:], brand)
	}
	return box
}

func performYucoreMediaUpload(t *testing.T, uploadRoot string, fileName string, declaredMimeType string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	t.Setenv("YUCORE_MEDIA_UPLOAD_DIR", uploadRoot)

	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+fileName+`"`)
	header.Set("Content-Type", declaredMimeType)
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/yucore/media/uploads", body)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())
	context.Set("id", 42)
	UploadYucoreMediaReference(context)
	return recorder
}

func TestYucoreMediaUploadCanonicalKinds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := yucoreMediaUploadTestContent()
	tests := []struct {
		name         string
		declaredMIME string
		detectedMIME string
		kind         string
		extension    string
	}{
		{name: "png", declaredMIME: "image/png", detectedMIME: "image/png", kind: "image", extension: ".png"},
		{name: "jpeg", declaredMIME: "image/jpeg", detectedMIME: "image/jpeg", kind: "image", extension: ".jpg"},
		{name: "webp", declaredMIME: "image/webp", detectedMIME: "image/webp", kind: "image", extension: ".webp"},
		{name: "gif", declaredMIME: "image/gif", detectedMIME: "image/gif", kind: "image", extension: ".gif"},
		{name: "mp4", declaredMIME: "video/mp4", detectedMIME: "video/mp4", kind: "video", extension: ".mp4"},
		{name: "mov", declaredMIME: "video/quicktime", detectedMIME: "video/quicktime", kind: "video", extension: ".mov"},
		{name: "mp3", declaredMIME: "audio/mpeg", detectedMIME: "audio/mpeg", kind: "audio", extension: ".mp3"},
		{name: "wav", declaredMIME: "audio/wav", detectedMIME: "audio/wav", kind: "audio", extension: ".wav"},
		{name: "x-wav alias", declaredMIME: "audio/x-wav", detectedMIME: "audio/wav", kind: "audio", extension: ".wav"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := yucoreMediaUploadPolicyFor(content[test.detectedMIME], test.declaredMIME)
			require.NoError(t, err)
			assert.Equal(t, test.kind, policy.Kind)
			assert.Equal(t, test.detectedMIME, policy.MIMEType)
			assert.Equal(t, test.extension, policy.Extension)

			uploadRoot := t.TempDir()
			recorder := performYucoreMediaUpload(t, uploadRoot, "spoofed.exe", test.declaredMIME, content[test.detectedMIME])
			assert.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
				Data    struct {
					Kind      string `json:"kind"`
					MIMEType  string `json:"mime_type"`
					CachedURL string `json:"cached_url"`
					Size      int64  `json:"size"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success, response.Message)
			assert.Equal(t, test.kind, response.Data.Kind)
			assert.Equal(t, test.detectedMIME, response.Data.MIMEType)
			assert.Equal(t, test.extension, filepath.Ext(response.Data.CachedURL))
			assert.Equal(t, int64(len(content[test.detectedMIME])), response.Data.Size)

			storedFiles, err := os.ReadDir(filepath.Join(uploadRoot, "42"))
			require.NoError(t, err)
			require.Len(t, storedFiles, 1)
			assert.Equal(t, test.extension, filepath.Ext(storedFiles[0].Name()))
			assert.False(t, strings.HasSuffix(storedFiles[0].Name(), ".part"))
			if runtime.GOOS != "windows" {
				info, err := storedFiles[0].Info()
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			}
		})
	}
}

func TestYucoreMediaUploadRejectsUnsafeEmptyAndSpoofedContent(t *testing.T) {
	content := yucoreMediaUploadTestContent()
	tests := []struct {
		name         string
		declaredMIME string
		content      []byte
	}{
		{name: "empty", declaredMIME: "image/png", content: nil},
		{name: "executable", declaredMIME: "application/octet-stream", content: []byte("MZ\x90\x00executable")},
		{name: "html", declaredMIME: "text/html", content: []byte("<!doctype html><script>alert(1)</script>")},
		{name: "scripted svg", declaredMIME: "image/svg+xml", content: []byte(`<svg><script>alert(1)</script></svg>`)},
		{name: "unknown binary", declaredMIME: "application/octet-stream", content: []byte{0x00, 0x01, 0x02, 0x03}},
		{name: "declared mime spoof", declaredMIME: "image/jpeg", content: content["image/png"]},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uploadRoot := t.TempDir()
			recorder := performYucoreMediaUpload(t, uploadRoot, "reference.bin", test.declaredMIME, test.content)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.False(t, response.Success)
			assert.NotEmpty(t, response.Message)
			entries, err := os.ReadDir(uploadRoot)
			require.NoError(t, err)
			assert.Empty(t, entries)
		})
	}
}

func TestYucoreMediaUploadValidatesMP3Structure(t *testing.T) {
	validTests := []struct {
		name    string
		content []byte
	}{
		{name: "ID3 followed by audio", content: yucoreMediaUploadTestContent()["audio/mpeg"]},
		{name: "two consecutive MPEG frames", content: yucoreMediaTestMPEGFrames(2)},
	}
	for _, test := range validTests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := yucoreMediaUploadPolicyFor(test.content, "audio/mpeg")
			require.NoError(t, err)
			assert.Equal(t, "audio", policy.Kind)
			assert.Equal(t, "audio/mpeg", policy.MIMEType)
			assert.Equal(t, ".mp3", policy.Extension)
		})
	}

	invalidTests := []struct {
		name    string
		content []byte
	}{
		{name: "ID3 script prefix", content: []byte("ID3<script>alert(1)</script>")},
		{name: "truncated ID3 header", content: []byte("ID3\x04\x00")},
		{name: "invalid ID3 version", content: []byte("ID3\xff\x00\x00\x00\x00\x00\x00")},
		{name: "invalid synchsafe size", content: []byte("ID3\x04\x00\x00\x80\x00\x00\x00")},
		{name: "ID3 tag exceeds prefix", content: []byte("ID3\x04\x00\x00\x00\x00\x01\x00payload")},
		{name: "ID3 without audio", content: []byte("ID3\x04\x00\x00\x00\x00\x00\x04TESTarbitrary")},
		{name: "single MPEG sync", content: []byte{0xff, 0xfb, 0x90, 0x64}},
		{name: "one MPEG frame plus arbitrary data", content: append(yucoreMediaTestMPEGFrames(1), []byte("not another frame")...)},
	}
	for _, test := range invalidTests {
		t.Run(test.name, func(t *testing.T) {
			_, err := yucoreMediaUploadPolicyFor(test.content, "audio/mpeg")
			require.Error(t, err)
		})
	}
}

func TestYucoreMediaUploadAcceptsBoundedLegitimateMP3Prefixes(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
	}{
		{name: "bounded ID3 tag", content: yucoreMediaTestID3Audio(600, yucoreMediaTestMPEGFrames(2))},
		{name: "high bitrate frames", content: yucoreMediaTestMPEGFramesWithHeader(2, 1044, []byte{0xff, 0xfb, 0xe0, 0x64})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := performYucoreMediaUpload(t, t.TempDir(), "reference.mp3", "audio/mpeg", test.content)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success, response.Message)
		})
	}
}

func TestYucoreMediaUploadValidatesFTYPStructureAndBrands(t *testing.T) {
	validTests := []struct {
		name     string
		content  []byte
		mimeType string
	}{
		{name: "MP4 major brand", content: yucoreMediaTestFTYP("isom", "mp41"), mimeType: "video/mp4"},
		{name: "MOV major brand", content: yucoreMediaTestFTYP("qt  ", "qt  "), mimeType: "video/quicktime"},
		{name: "MP4 compatible brand", content: yucoreMediaTestFTYP("zzzz", "isom"), mimeType: "video/mp4"},
		{name: "MOV compatible brand", content: yucoreMediaTestFTYP("zzzz", "qt  "), mimeType: "video/quicktime"},
	}
	for _, test := range validTests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := yucoreMediaUploadPolicyFor(test.content, test.mimeType)
			require.NoError(t, err)
			assert.Equal(t, test.mimeType, policy.MIMEType)
		})
	}

	const ftypSniffLimit = 512
	oversizedBox := make([]byte, ftypSniffLimit)
	binary.BigEndian.PutUint32(oversizedBox[:4], uint32(ftypSniffLimit+4))
	copy(oversizedBox[4:8], "ftyp")
	copy(oversizedBox[8:12], "isom")
	invalidTests := []struct {
		name    string
		content []byte
	}{
		{name: "twelve byte box", content: []byte("\x00\x00\x00\x0cftypisom")},
		{name: "truncated box", content: []byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00")},
		{name: "oversized box", content: oversizedBox},
		{name: "misaligned compatible brands", content: []byte("\x00\x00\x00\x11ftypisom\x00\x00\x00\x00x")},
		{name: "unknown brands", content: yucoreMediaTestFTYP("zzzz", "yyyy")},
	}
	for _, test := range invalidTests {
		t.Run(test.name, func(t *testing.T) {
			_, err := yucoreMediaUploadPolicyFor(test.content, "video/mp4")
			require.Error(t, err)
		})
	}
}

func TestYucoreMediaUploadSizePolicyAndCleanup(t *testing.T) {
	content := yucoreMediaUploadTestContent()
	tests := []struct {
		mimeType string
		want     int64
	}{
		{mimeType: "image/png", want: 25 << 20},
		{mimeType: "audio/mpeg", want: 25 << 20},
		{mimeType: "video/mp4", want: 100 << 20},
	}
	for _, test := range tests {
		policy, err := yucoreMediaUploadPolicyFor(content[test.mimeType], test.mimeType)
		require.NoError(t, err)
		assert.Equal(t, test.want, policy.MaxBytes)
	}
	assert.GreaterOrEqual(t, yucoreMediaUploadRequestMaxBytes, int64((100<<20)+(1<<20)))

	ownerDir := filepath.Join(t.TempDir(), "42")
	finalPath := filepath.Join(ownerDir, "oversized.png")
	_, err := storeYucoreMediaUpload(bytes.NewReader([]byte("12345")), finalPath, 4)
	require.Error(t, err)
	entries, err := os.ReadDir(ownerDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
	_, err = os.Stat(finalPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestServeYucoreMediaUploadedReferencePreservesOwnerAndSignatureAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uploadRoot := t.TempDir()
	t.Setenv("YUCORE_MEDIA_UPLOAD_DIR", uploadRoot)
	ownerID := 42
	fileName := "ref_test.png"
	fullPath := filepath.Join(uploadRoot, "42", fileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o700))
	require.NoError(t, os.WriteFile(fullPath, yucoreMediaUploadTestContent()["image/png"], 0o600))

	serve := func(requestURL string, authenticatedUserID int) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodGet, requestURL, nil)
		context.Params = gin.Params{{Key: "user_id", Value: "42"}, {Key: "file", Value: fileName}}
		context.Set("id", authenticatedUserID)
		ServeYucoreMediaUploadedReference(context)
		return recorder
	}

	ownerResponse := serve("/api/yucore/media/uploads/42/"+fileName, ownerID)
	assert.Equal(t, http.StatusOK, ownerResponse.Code)
	assert.Equal(t, "nosniff", ownerResponse.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "private, max-age=86400", ownerResponse.Header().Get("Cache-Control"))
	assert.Equal(t, yucoreMediaUploadTestContent()["image/png"], ownerResponse.Body.Bytes())

	signedResponse := serve("/api/yucore/media/uploads/42/"+fileName+"?sig="+yucoreMediaUploadSignature(ownerID, fileName), 7)
	assert.Equal(t, http.StatusOK, signedResponse.Code)

	unauthorizedResponse := serve("/api/yucore/media/uploads/42/"+fileName, 7)
	assert.Equal(t, http.StatusUnauthorized, unauthorizedResponse.Code)
}

func TestServeYucoreMediaTaskAssetSelectsPrivateThumbnailSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamRequests := make([]string, 0, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests = append(upstreamRequests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/octet-stream")
		switch r.URL.Path {
		case "/content":
			_, _ = w.Write([]byte("content-bytes"))
		case "/thumbnail":
			_, _ = w.Write([]byte("thumbnail-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:yucore_media_thumbnail_proxy?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.YucoreMediaTask{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	persistedAssets, err := common.Marshal([]map[string]any{{
		"id":               "yu_thumbnail_asset_0",
		"kind":             "video",
		"url":              "/api/yucore/media/tasks/yu_thumbnail/assets/0",
		"thumb_url":        "/api/yucore/media/tasks/yu_thumbnail/assets/0?variant=thumbnail",
		"source_url":       upstream.URL + "/content?signature=content",
		"source_thumb_url": upstream.URL + "/thumbnail?signature=thumb",
		"label":            "thumbnail result",
		"mime_type":        "video/mp4",
	}})
	require.NoError(t, err)
	task := &model.YucoreMediaTask{
		TaskId: "yu_thumbnail", UserId: 42, Kind: "video", ModelId: "video-model",
		Status: model.YucoreMediaTaskStatusCompleted, Assets: model.YucoreMediaAssets(persistedAssets),
		Metadata: `{"adapter":"openai-compatible"}`, CreatedTime: common.GetTimestamp(), UpdatedTime: common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(task).Error)

	serve := func(requestURL string, userID int) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodGet, requestURL, nil)
		context.Params = gin.Params{{Key: "task_id", Value: task.TaskId}, {Key: "index", Value: "0"}}
		context.Set("id", userID)
		ServeYucoreMediaTaskAsset(context)
		return recorder
	}

	contentResponse := serve("/api/yucore/media/tasks/yu_thumbnail/assets/0", 42)
	assert.Equal(t, http.StatusOK, contentResponse.Code)
	assert.Equal(t, "content-bytes", contentResponse.Body.String())
	assert.Equal(t, "private, max-age=300", contentResponse.Header().Get("Cache-Control"))
	thumbnailResponse := serve("/api/yucore/media/tasks/yu_thumbnail/assets/0?variant=thumbnail", 42)
	assert.Equal(t, http.StatusOK, thumbnailResponse.Code)
	assert.Equal(t, "thumbnail-bytes", thumbnailResponse.Body.String())
	assert.Equal(t, "private, max-age=300", thumbnailResponse.Header().Get("Cache-Control"))
	invalidVariantResponse := serve("/api/yucore/media/tasks/yu_thumbnail/assets/0?variant=thumbnail-extra", 42)
	assert.Equal(t, http.StatusOK, invalidVariantResponse.Code)
	assert.Equal(t, "content-bytes", invalidVariantResponse.Body.String())
	assert.Equal(t, []string{"/content?signature=content", "/thumbnail?signature=thumb", "/content?signature=content"}, upstreamRequests)

	unauthorizedResponse := serve("/api/yucore/media/tasks/yu_thumbnail/assets/0?variant=thumbnail", 0)
	assert.Equal(t, http.StatusUnauthorized, unauthorizedResponse.Code)
	nonOwnerResponse := serve("/api/yucore/media/tasks/yu_thumbnail/assets/0?variant=thumbnail", 7)
	assert.Equal(t, http.StatusNotFound, nonOwnerResponse.Code)

	responseJSON, err := common.Marshal(buildYucoreMediaTaskResponse(task))
	require.NoError(t, err)
	assert.NotContains(t, string(responseJSON), upstream.URL)
	assert.NotContains(t, string(responseJSON), "signature=thumb")
	assert.Contains(t, string(responseJSON), `"thumb_url":"/api/yucore/media/tasks/yu_thumbnail/assets/0?variant=thumbnail"`)
}
