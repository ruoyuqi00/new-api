package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

func TestYucoreMediaTaskAssetsMigrationType(t *testing.T) {
	mysqlDB, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:password@tcp(127.0.0.1:3306)/newapi",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
	})
	require.NoError(t, err)

	statement := &gorm.Statement{DB: mysqlDB}
	require.NoError(t, statement.Parse(&YucoreMediaTask{}))
	assetsField := statement.Schema.LookUpField("Assets")
	require.NotNil(t, assetsField)
	assert.Equal(t, "LONGTEXT", mysqlDB.Migrator().FullDataTypeOf(assetsField).SQL)

	dummyDB, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, "TEXT", YucoreMediaAssets("").GormDBDataType(dummyDB, assetsField))
}

func TestParseYucoreMediaUAGAllowlist(t *testing.T) {
	got := parseYucoreMediaUAGAllowlist(" GPT ;flow\nimg-v3,, ")
	for _, want := range []string{"gpt", "flow", "img-v3"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("allowlist missing %q: %#v", want, got)
		}
	}
	if _, ok := got["GPT"]; ok {
		t.Fatalf("allowlist should normalize to lowercase: %#v", got)
	}

	jsonGot := parseYucoreMediaUAGAllowlist(`["GPT","img-real"]`)
	for _, want := range []string{"gpt", "img-real"} {
		if _, ok := jsonGot[want]; !ok {
			t.Fatalf("json allowlist missing %q: %#v", want, jsonGot)
		}
	}
}

func TestParseYucoreMediaUAGModelMap(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want map[string]string
	}{
		{
			name: "pairs",
			raw:  "gpt-image-2=img-v3, grok-imagine-image-quality=img-real",
			want: map[string]string{
				"gpt-image-2":                "img-v3",
				"grok-imagine-image-quality": "img-real",
			},
		},
		{
			name: "json",
			raw:  `{"gpt-image-2":"img-v3"}`,
			want: map[string]string{"gpt-image-2": "img-v3"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := parseYucoreMediaUAGModelMap(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d: %#v", len(got), len(tt.want), got)
			}
			for key, want := range tt.want {
				if got[key] != want {
					t.Fatalf("map[%q] = %q, want %q", key, got[key], want)
				}
			}
		})
	}
}

func TestYucoreMediaUAGModelID(t *testing.T) {
	config := yucoreMediaAdapterConfig{
		UAGModelMap: map[string]string{"GPT-IMAGE-2": "img-v3"},
	}
	if got := yucoreMediaUAGModelID(config, "gpt-image-2"); got != "img-v3" {
		t.Fatalf("mapped model = %q, want img-v3", got)
	}
	if got := yucoreMediaUAGModelID(config, "img-real"); got != "img-real" {
		t.Fatalf("unmapped model = %q, want img-real", got)
	}
}

func TestYucoreMediaAdapterInfoUAGUnverifiedByDefault(t *testing.T) {
	t.Setenv("YUCORE_MEDIA_ADAPTER", "uag-proxy")
	t.Setenv("YUCORE_MEDIA_BASE_URL", "http://uag.local")
	t.Setenv("YUCORE_MEDIA_API_KEY", "token")
	t.Setenv("YUCORE_MEDIA_UPSTREAM_VERIFIED", "")

	info := GetYucoreMediaAdapterInfo()
	if !info.Configured {
		t.Fatal("expected UAG adapter to be configured")
	}
	if info.UpstreamVerified {
		t.Fatal("expected UAG upstream to remain unverified by default")
	}
	if info.UpstreamVerificationStatus != "unverified" {
		t.Fatalf("verification status = %q, want unverified", info.UpstreamVerificationStatus)
	}
	if info.Message == "" {
		t.Fatal("expected unverified UAG adapter to include a message")
	}
	if info.RealWorkflowReady {
		t.Fatal("expected unverified UAG adapter to be blocked for real workflow completion")
	}
	if len(info.VerificationBlockers) == 0 {
		t.Fatal("expected verification blockers for unverified UAG adapter")
	}
}

func TestYucoreMediaAdapterInfoUAGExplicitlyVerified(t *testing.T) {
	t.Setenv("YUCORE_MEDIA_ADAPTER", "uag-proxy")
	t.Setenv("YUCORE_MEDIA_BASE_URL", "http://uag.local")
	t.Setenv("YUCORE_MEDIA_API_KEY", "token")
	t.Setenv("YUCORE_MEDIA_UPSTREAM_VERIFIED", "true")

	info := GetYucoreMediaAdapterInfo()
	if !info.UpstreamVerified {
		t.Fatal("expected explicit verification flag to mark UAG upstream verified")
	}
	if info.UpstreamVerificationStatus != "verified" {
		t.Fatalf("verification status = %q, want verified", info.UpstreamVerificationStatus)
	}
	if info.RealWorkflowReady {
		t.Fatal("expected explicit verification without require_real_assets to remain blocked")
	}
	if !yucoreMediaTestHasBlocker(info.VerificationBlockers, "require_real_assets") {
		t.Fatalf("expected require_real_assets blocker, got %#v", info.VerificationBlockers)
	}
}

func TestYucoreMediaAdapterInfoRealWorkflowReady(t *testing.T) {
	t.Setenv("YUCORE_MEDIA_ADAPTER", "uag-proxy")
	t.Setenv("YUCORE_MEDIA_BASE_URL", "http://uag.local")
	t.Setenv("YUCORE_MEDIA_API_KEY", "token")
	t.Setenv("YUCORE_MEDIA_REQUIRE_REAL_ASSETS", "true")
	t.Setenv("YUCORE_MEDIA_UPSTREAM_VERIFIED", "true")

	info := GetYucoreMediaAdapterInfo()
	if !info.RealWorkflowReady {
		t.Fatalf("expected real workflow ready, got blockers %#v", info.VerificationBlockers)
	}
	if len(info.VerificationBlockers) != 0 {
		t.Fatalf("expected no verification blockers, got %#v", info.VerificationBlockers)
	}
}

func yucoreMediaTestHasBlocker(blockers []string, needle string) bool {
	for _, blocker := range blockers {
		if strings.Contains(blocker, needle) {
			return true
		}
	}
	return false
}

func TestYucoreMediaAdapterConfigPrefersAdminOptions(t *testing.T) {
	t.Setenv("YUCORE_MEDIA_ADAPTER", "mock")
	t.Setenv("YUCORE_MEDIA_BASE_URL", "http://env.local")
	t.Setenv("YUCORE_MEDIA_API_KEY", "env-token")
	t.Setenv("YUCORE_MEDIA_REQUIRE_REAL_ASSETS", "false")
	t.Setenv("YUCORE_MEDIA_UAG_ALLOWED_MODELS", "env-model")

	common.OptionMapRWMutex.Lock()
	original := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.adapter":               "uag-proxy",
		"yucore_media.base_url":              "http://admin.local/",
		"yucore_media.api_key":               "admin-token",
		"yucore_media.timeout_seconds":       "33",
		"yucore_media.require_real_assets":   "true",
		"yucore_media.uag_model_map":         "gpt-image-2=img-v3",
		"yucore_media.uag_allowed_models":    "img-v3",
		"yucore_media.uag_allowed_providers": "gpt",
		"yucore_media.upstream_verified":     "true",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = original
		common.OptionMapRWMutex.Unlock()
	})

	config := getYucoreMediaAdapterConfig()
	if config.Adapter != YucoreMediaAdapterUAGProxy {
		t.Fatalf("adapter = %q, want %q", config.Adapter, YucoreMediaAdapterUAGProxy)
	}
	if config.BaseURL != "http://admin.local" {
		t.Fatalf("base url = %q, want http://admin.local", config.BaseURL)
	}
	if config.APIKey != "admin-token" {
		t.Fatalf("api key = %q, want admin-token", config.APIKey)
	}
	if config.TimeoutSeconds != 33 {
		t.Fatalf("timeout = %d, want 33", config.TimeoutSeconds)
	}
	if !config.RequireRealAssets || !config.UpstreamVerified {
		t.Fatalf("expected admin booleans to be true: %#v", config)
	}
	if !yucoreMediaUAGAllowed(config.UAGAllowedModels, "img-v3") {
		t.Fatalf("expected admin model allowlist to include img-v3")
	}
	if yucoreMediaUAGAllowed(config.UAGAllowedModels, "env-model") {
		t.Fatalf("expected admin model allowlist to override env model")
	}
	if got := yucoreMediaUAGModelID(config, "gpt-image-2"); got != "img-v3" {
		t.Fatalf("mapped model = %q, want img-v3", got)
	}
}

func TestYucoreMediaUAGModelRow(t *testing.T) {
	row := map[string]any{
		"model_code":     "img-v3",
		"name":           "General Image",
		"kind":           "image",
		"provider":       "gpt",
		"upstream_model": "gpt-image",
		"unit_points":    float64(400),
		"enabled":        true,
	}

	model, ok := yucoreMediaUAGModelRow(row, yucoreMediaAdapterConfig{})
	if !ok {
		t.Fatal("expected image model row to be accepted")
	}
	if model["id"] != "img-v3" {
		t.Fatalf("id = %v, want img-v3", model["id"])
	}
	if model["kind"] != "image" {
		t.Fatalf("kind = %v, want image", model["kind"])
	}
	if model["source"] != "uag-proxy" {
		t.Fatalf("source = %v, want uag-proxy", model["source"])
	}
	pricing, ok := model["pricing"].(map[string]any)
	if !ok || pricing["unit_points"] != 400 {
		t.Fatalf("pricing = %#v, want unit_points 400", model["pricing"])
	}

	if _, ok := yucoreMediaUAGModelRow(map[string]any{"model_code": "text", "kind": "text"}, yucoreMediaAdapterConfig{}); ok {
		t.Fatal("text models should be filtered out")
	}
	if _, ok := yucoreMediaUAGModelRow(map[string]any{"model_code": "img-disabled", "kind": "image", "enabled": false}, yucoreMediaAdapterConfig{}); ok {
		t.Fatal("disabled models should be filtered out")
	}
}

func TestYucoreMediaUAGModelRowAllowlist(t *testing.T) {
	row := map[string]any{
		"model_code": "img-v3",
		"name":       "General Image",
		"kind":       "image",
		"provider":   "gpt",
		"enabled":    true,
	}
	config := yucoreMediaAdapterConfig{
		UAGAllowedProviders: parseYucoreMediaUAGAllowlist("GPT"),
		UAGAllowedModels:    parseYucoreMediaUAGAllowlist("img-v3"),
	}
	if _, ok := yucoreMediaUAGModelRow(row, config); !ok {
		t.Fatal("expected allowed model row to be accepted")
	}

	providerBlocked := yucoreMediaAdapterConfig{
		UAGAllowedProviders: parseYucoreMediaUAGAllowlist("flow"),
	}
	if _, ok := yucoreMediaUAGModelRow(row, providerBlocked); ok {
		t.Fatal("expected non-allowed provider to be filtered out")
	}

	modelBlocked := yucoreMediaAdapterConfig{
		UAGAllowedModels: parseYucoreMediaUAGAllowlist("img-real"),
	}
	if _, ok := yucoreMediaUAGModelRow(row, modelBlocked); ok {
		t.Fatal("expected non-allowed model to be filtered out")
	}
}

func TestBuildYucoreCanvasAgentRunActions(t *testing.T) {
	task := &YucoreMediaTask{
		TaskId:  "yu_done",
		Status:  YucoreMediaTaskStatusCompleted,
		ModelId: "img-v3",
		Assets:  `[{"id":"asset_1","kind":"image","url":"/asset.png","thumb_url":"/thumb.png"}]`,
	}
	got := buildYucoreCanvasAgentRunActions(`[{"tool":"canvas_plan","status":"completed"},{"tool":"canvas_run_generation","status":"running","task_id":"yu_done"}]`, task)
	var rows []map[string]any
	if err := json.Unmarshal([]byte(got), &rows); err != nil {
		t.Fatalf("actions are not valid json: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(actions) = %d, want 3: %s", len(rows), got)
	}
	if rows[1]["status"] != YucoreCanvasAgentRunStatusCompleted {
		t.Fatalf("generation status = %v, want completed", rows[1]["status"])
	}
	if rows[2]["tool"] != "canvas_apply_result" {
		t.Fatalf("last action tool = %v, want canvas_apply_result", rows[2]["tool"])
	}

	again := buildYucoreCanvasAgentRunActions(got, task)
	var againRows []map[string]any
	if err := json.Unmarshal([]byte(again), &againRows); err != nil {
		t.Fatalf("second actions are not valid json: %v", err)
	}
	applyCount := 0
	for _, row := range againRows {
		if row["tool"] == "canvas_apply_result" {
			applyCount++
		}
	}
	if applyCount != 1 {
		t.Fatalf("apply action count = %d, want 1: %s", applyCount, again)
	}
}

func TestApplyYucoreCanvasMediaTaskSnapshotBackflow(t *testing.T) {
	task := &YucoreMediaTask{
		TaskId:  "yu_done",
		Status:  YucoreMediaTaskStatusCompleted,
		Kind:    "image",
		ModelId: "img-v3",
		Prompt:  "make a luminous product photo",
		Assets:  `[{"id":"asset_1","kind":"image","url":"/api/yucore/media/tasks/yu_done/assets/0","thumb_url":"/api/yucore/media/tasks/yu_done/assets/0","label":"img-v3 result 1"}]`,
	}
	snapshot := `{"nodes":[{"id":"prompt_1","data":{"status":"planned"}},{"id":"task_1","data":{"label":"Generating","status":"running"},"style":{"padding":14}}],"edges":[]}`
	got, changed, err := applyYucoreCanvasMediaTaskSnapshotBackflow(snapshot, task, "prompt_1", "task_1")
	if err != nil {
		t.Fatalf("snapshot backflow returned error: %v", err)
	}
	if !changed {
		t.Fatal("expected snapshot to change")
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(got), &root); err != nil {
		t.Fatalf("snapshot is not valid json: %v", err)
	}
	nodes := root["nodes"].([]any)
	promptData := nodes[0].(map[string]any)["data"].(map[string]any)
	if promptData["status"] != "linked yu_done / completed" {
		t.Fatalf("prompt status = %v", promptData["status"])
	}
	taskNode := nodes[1].(map[string]any)
	taskData := taskNode["data"].(map[string]any)
	if taskData["resultTaskId"] != "yu_done" {
		t.Fatalf("resultTaskId = %v, want yu_done", taskData["resultTaskId"])
	}
	if taskData["assetUrl"] != "/api/yucore/media/tasks/yu_done/assets/0" {
		t.Fatalf("assetUrl = %v", taskData["assetUrl"])
	}
	style := taskNode["style"].(map[string]any)
	if style["padding"] != float64(0) {
		t.Fatalf("padding = %v, want 0", style["padding"])
	}

	again, changedAgain, err := applyYucoreCanvasMediaTaskSnapshotBackflow(got, task, "prompt_1", "task_1")
	if err != nil {
		t.Fatalf("second snapshot backflow returned error: %v", err)
	}
	if changedAgain {
		t.Fatalf("expected second snapshot backflow to be idempotent: %s", again)
	}
}

func TestHideYucoreMediaTaskInRealAssetMode(t *testing.T) {
	config := yucoreMediaAdapterConfig{RequireRealAssets: true}
	mockTask := &YucoreMediaTask{Metadata: `{}`}
	if !hideYucoreMediaTaskInRealAssetMode(mockTask, config) {
		t.Fatal("expected legacy mock task to be hidden when real assets are required")
	}
	uagTask := &YucoreMediaTask{Metadata: `{"adapter":"uag-proxy"}`}
	if hideYucoreMediaTaskInRealAssetMode(uagTask, config) {
		t.Fatal("expected UAG task to remain visible when real assets are required")
	}
	if hideYucoreMediaTaskInRealAssetMode(mockTask, yucoreMediaAdapterConfig{}) {
		t.Fatal("expected mock task to remain visible when real assets are not required")
	}
}
