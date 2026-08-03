package controller

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	maxYucoreCanvasTitleLength        = 128
	maxYucoreCanvasDescriptionLength  = 512
	maxYucoreCanvasModuleLength       = 64
	maxYucoreCanvasSnapshotBytes      = 512 * 1024
	maxYucoreCanvasAgentPromptLength  = 6000
	maxYucoreCanvasAgentSummaryLength = 512
	maxYucoreCanvasAgentActionsBytes  = 256 * 1024
	maxYucoreCanvasNodeIdLength       = 128
)

type yucoreCanvasRequest struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Module      string          `json:"module"`
	Snapshot    json.RawMessage `json:"snapshot"`
	Viewport    json.RawMessage `json:"viewport"`
	Autosave    bool            `json:"autosave"`
}

type yucoreCanvasResponse struct {
	Id          int             `json:"id"`
	UserId      int             `json:"user_id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Module      string          `json:"module"`
	Snapshot    json.RawMessage `json:"snapshot"`
	Viewport    json.RawMessage `json:"viewport"`
	Revision    int             `json:"revision"`
	CreatedTime int64           `json:"created_time"`
	UpdatedTime int64           `json:"updated_time"`
}

type yucoreCanvasVersionResponse struct {
	Id          int             `json:"id"`
	CanvasId    int             `json:"canvas_id"`
	UserId      int             `json:"user_id"`
	Revision    int             `json:"revision"`
	Title       string          `json:"title"`
	Module      string          `json:"module"`
	Snapshot    json.RawMessage `json:"snapshot"`
	Viewport    json.RawMessage `json:"viewport"`
	CreatedTime int64           `json:"created_time"`
}

type yucoreCanvasAgentRunRequest struct {
	Mode         string          `json:"mode"`
	Prompt       string          `json:"prompt"`
	Status       string          `json:"status"`
	Summary      string          `json:"summary"`
	Actions      json.RawMessage `json:"actions"`
	ResultTaskId string          `json:"result_task_id"`
}

type yucoreCanvasAgentRunResponse struct {
	Id           int             `json:"id"`
	RunId        string          `json:"run_id"`
	UserId       int             `json:"user_id"`
	CanvasId     int             `json:"canvas_id"`
	Mode         string          `json:"mode"`
	Prompt       string          `json:"prompt"`
	Status       string          `json:"status"`
	Summary      string          `json:"summary"`
	Actions      json.RawMessage `json:"actions"`
	ResultTaskId string          `json:"result_task_id"`
	CreatedTime  int64           `json:"created_time"`
	UpdatedTime  int64           `json:"updated_time"`
}

type yucoreCanvasAgentExecuteRequest struct {
	Mode              string          `json:"mode"`
	Prompt            string          `json:"prompt"`
	Group             string          `json:"group"`
	Kind              string          `json:"kind"`
	MediaMode         string          `json:"media_mode"`
	ModelId           string          `json:"model_id"`
	NegativePrompt    string          `json:"negative_prompt"`
	AspectRatio       string          `json:"aspect_ratio"`
	Size              string          `json:"size"`
	Quality           string          `json:"quality"`
	Format            string          `json:"format"`
	Count             int             `json:"count"`
	SessionId         string          `json:"session_id"`
	Inputs            json.RawMessage `json:"inputs"`
	Metadata          json.RawMessage `json:"metadata"`
	AgentPromptNodeId string          `json:"agent_prompt_node_id"`
	AgentTaskNodeId   string          `json:"agent_task_node_id"`
}

type yucoreCanvasAgentExecuteResponse struct {
	Run      yucoreCanvasAgentRunResponse `json:"run"`
	Task     yucoreMediaTaskResponse      `json:"task"`
	Identity yucoreCanvasIdentityResponse `json:"identity"`
}

type yucoreCanvasIdentityResponse struct {
	UserId          int               `json:"user_id"`
	Username        string            `json:"username"`
	IdentitySession string            `json:"identity_session"`
	IdentityToken   string            `json:"identity_token"`
	IssuedAt        int64             `json:"issued_at"`
	ExpiresAt       int64             `json:"expires_at"`
	Scopes          []string          `json:"scopes"`
	StorageKeys     map[string]string `json:"storage_keys"`
}

func parseYucoreCanvasId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid canvas id")
		return 0, false
	}
	return id, true
}

func normalizeYucoreRawJSON(raw json.RawMessage, fallback string) (string, error) {
	if len(raw) == 0 {
		return fallback, nil
	}
	if len(raw) > maxYucoreCanvasSnapshotBytes {
		return "", errors.New("canvas snapshot is too large")
	}
	if !json.Valid(raw) {
		return "", errors.New("canvas snapshot must be valid JSON")
	}
	return string(raw), nil
}

func buildYucoreCanvasFromRequest(req yucoreCanvasRequest, userId int) (*model.YucoreCanvas, error) {
	title := strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(title) > maxYucoreCanvasTitleLength {
		return nil, errors.New("canvas title is too long")
	}
	description := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(description) > maxYucoreCanvasDescriptionLength {
		return nil, errors.New("canvas description is too long")
	}
	module := strings.TrimSpace(req.Module)
	if utf8.RuneCountInString(module) > maxYucoreCanvasModuleLength {
		return nil, errors.New("canvas module is too long")
	}
	snapshot, err := normalizeYucoreRawJSON(req.Snapshot, `{"nodes":[],"edges":[]}`)
	if err != nil {
		return nil, err
	}
	viewport, err := normalizeYucoreRawJSON(req.Viewport, `{}`)
	if err != nil {
		return nil, err
	}
	return &model.YucoreCanvas{
		UserId:      userId,
		Title:       title,
		Description: description,
		Module:      module,
		Snapshot:    snapshot,
		Viewport:    viewport,
	}, nil
}

func rawYucoreJSON(value string, fallback string) json.RawMessage {
	if value == "" || !json.Valid([]byte(value)) {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(value)
}

func buildYucoreCanvasResponse(canvas *model.YucoreCanvas) yucoreCanvasResponse {
	return yucoreCanvasResponse{
		Id:          canvas.Id,
		UserId:      canvas.UserId,
		Title:       canvas.Title,
		Description: canvas.Description,
		Module:      canvas.Module,
		Snapshot:    rawYucoreJSON(canvas.Snapshot, `{"nodes":[],"edges":[]}`),
		Viewport:    rawYucoreJSON(canvas.Viewport, `{}`),
		Revision:    canvas.Revision,
		CreatedTime: canvas.CreatedTime,
		UpdatedTime: canvas.UpdatedTime,
	}
}

func buildYucoreCanvasResponses(canvases []*model.YucoreCanvas) []yucoreCanvasResponse {
	responses := make([]yucoreCanvasResponse, 0, len(canvases))
	for _, canvas := range canvases {
		responses = append(responses, buildYucoreCanvasResponse(canvas))
	}
	return responses
}

func buildYucoreCanvasVersionResponse(version *model.YucoreCanvasVersion) yucoreCanvasVersionResponse {
	return yucoreCanvasVersionResponse{
		Id:          version.Id,
		CanvasId:    version.CanvasId,
		UserId:      version.UserId,
		Revision:    version.Revision,
		Title:       version.Title,
		Module:      version.Module,
		Snapshot:    rawYucoreJSON(version.Snapshot, `{"nodes":[],"edges":[]}`),
		Viewport:    rawYucoreJSON(version.Viewport, `{}`),
		CreatedTime: version.CreatedTime,
	}
}

func buildYucoreCanvasAgentRunFromRequest(req yucoreCanvasAgentRunRequest, canvasId int, userId int) (*model.YucoreCanvasAgentRun, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("agent prompt is required")
	}
	if utf8.RuneCountInString(prompt) > maxYucoreCanvasAgentPromptLength {
		return nil, errors.New("agent prompt is too long")
	}
	summary := strings.TrimSpace(req.Summary)
	if utf8.RuneCountInString(summary) > maxYucoreCanvasAgentSummaryLength {
		return nil, errors.New("agent summary is too long")
	}
	actions, err := normalizeYucoreRawJSON(req.Actions, `[]`)
	if err != nil {
		return nil, errors.New(strings.Replace(err.Error(), "canvas snapshot", "agent actions", 1))
	}
	if len(actions) > maxYucoreCanvasAgentActionsBytes {
		return nil, errors.New("agent actions are too large")
	}
	status := strings.TrimSpace(req.Status)
	switch status {
	case "", model.YucoreCanvasAgentRunStatusQueued, model.YucoreCanvasAgentRunStatusRunning, model.YucoreCanvasAgentRunStatusCompleted, model.YucoreCanvasAgentRunStatusFailed:
	default:
		return nil, errors.New("unsupported agent run status")
	}
	return &model.YucoreCanvasAgentRun{
		UserId:       userId,
		CanvasId:     canvasId,
		Mode:         req.Mode,
		Prompt:       prompt,
		Status:       status,
		Summary:      summary,
		Actions:      actions,
		ResultTaskId: strings.TrimSpace(req.ResultTaskId),
	}, nil
}

func buildYucoreCanvasAgentRunResponse(run *model.YucoreCanvasAgentRun) yucoreCanvasAgentRunResponse {
	return yucoreCanvasAgentRunResponse{
		Id:           run.Id,
		RunId:        run.RunId,
		UserId:       run.UserId,
		CanvasId:     run.CanvasId,
		Mode:         run.Mode,
		Prompt:       run.Prompt,
		Status:       run.Status,
		Summary:      run.Summary,
		Actions:      rawYucoreJSON(run.Actions, `[]`),
		ResultTaskId: run.ResultTaskId,
		CreatedTime:  run.CreatedTime,
		UpdatedTime:  run.UpdatedTime,
	}
}

func buildYucoreCanvasAgentRunResponses(runs []*model.YucoreCanvasAgentRun) []yucoreCanvasAgentRunResponse {
	responses := make([]yucoreCanvasAgentRunResponse, 0, len(runs))
	for _, run := range runs {
		responses = append(responses, buildYucoreCanvasAgentRunResponse(run))
	}
	return responses
}

func normalizeYucoreCanvasNodeId(value string, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if utf8.RuneCountInString(value) > maxYucoreCanvasNodeIdLength {
		return "", fmt.Errorf("%s is too long", label)
	}
	return value, nil
}

func mergeYucoreCanvasObjectJSON(value string, patch map[string]any) (string, error) {
	metadata := map[string]any{}
	if strings.TrimSpace(value) != "" {
		if err := json.Unmarshal([]byte(value), &metadata); err != nil {
			return "", err
		}
	}
	for key, val := range patch {
		metadata[key] = val
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func buildYucoreCanvasAgentExecuteMediaRequest(req yucoreCanvasAgentExecuteRequest) yucoreMediaTaskRequest {
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "image"
	}
	mediaMode := strings.TrimSpace(req.MediaMode)
	if mediaMode == "" {
		if kind == "video" {
			mediaMode = "text-to-video"
		} else {
			mediaMode = "text-to-image"
		}
	}
	return yucoreMediaTaskRequest{
		Group:          req.Group,
		Kind:           kind,
		Mode:           mediaMode,
		ModelId:        req.ModelId,
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		AspectRatio:    req.AspectRatio,
		Size:           req.Size,
		Quality:        req.Quality,
		Format:         req.Format,
		Count:          req.Count,
		SessionId:      req.SessionId,
		Inputs:         req.Inputs,
		Metadata:       req.Metadata,
	}
}

func buildYucoreCanvasAgentExecuteActions(req yucoreCanvasAgentExecuteRequest, task *model.YucoreMediaTask, identity yucoreCanvasIdentityResponse) (json.RawMessage, error) {
	actions := []map[string]any{
		{
			"tool":       "canvas_identity_bridge",
			"status":     "completed",
			"session":    identity.IdentitySession,
			"expires_at": identity.ExpiresAt,
		},
		{
			"tool":   "canvas_get_state",
			"status": "completed",
		},
		{
			"tool":     "canvas_create_generation_flow",
			"status":   "completed",
			"node_ids": []string{req.AgentPromptNodeId, req.AgentTaskNodeId},
		},
		{
			"tool":     "canvas_run_generation",
			"status":   "queued",
			"task_id":  task.TaskId,
			"model_id": task.ModelId,
		},
	}
	raw, err := json.Marshal(actions)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func buildYucoreCanvasIdentity(userId int, username string) (yucoreCanvasIdentityResponse, error) {
	now := common.GetTimestamp()
	ttl := int64(common.GetEnvOrDefault("YUCORE_CANVAS_IDENTITY_TTL_SECONDS", 86400))
	if ttl <= 0 {
		ttl = 86400
	}
	expiresAt := now + ttl
	sessionSig := common.GenerateHMAC(fmt.Sprintf("yucore-canvas-session:%d:%s", userId, username))
	identitySession := "yc_session_" + sessionSig[:20]
	payload := map[string]any{
		"sub":     userId,
		"name":    username,
		"session": identitySession,
		"scope":   "canvas media agent",
		"iat":     now,
		"exp":     expiresAt,
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return yucoreCanvasIdentityResponse{}, err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(rawPayload)
	signature := common.GenerateHMAC("yucore-canvas-identity:" + encodedPayload)
	return yucoreCanvasIdentityResponse{
		UserId:          userId,
		Username:        username,
		IdentitySession: identitySession,
		IdentityToken:   "ycid_" + encodedPayload + "." + signature[:32],
		IssuedAt:        now,
		ExpiresAt:       expiresAt,
		Scopes: []string{
			"canvas:read",
			"canvas:write",
			"media:task:create",
			"agent:run:update",
		},
		StorageKeys: map[string]string{
			"identity_token":   "infinite-canvas:identity-token",
			"identity_session": "infinite-canvas:identity-session",
		},
	}, nil
}

func GetYucoreCanvasIdentity(c *gin.Context) {
	identity, err := buildYucoreCanvasIdentity(c.GetInt("id"), c.GetString("username"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, identity)
}

func ListYucoreCanvases(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	canvases, err := model.ListYucoreCanvases(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountYucoreCanvases(userId)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildYucoreCanvasResponses(canvases))
	common.ApiSuccess(c, pageInfo)
}

func CreateYucoreCanvas(c *gin.Context) {
	var req yucoreCanvasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	canvas, err := buildYucoreCanvasFromRequest(req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.CreateYucoreCanvas(canvas); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildYucoreCanvasResponse(canvas),
	})
}

func GetYucoreCanvas(c *gin.Context) {
	id, ok := parseYucoreCanvasId(c)
	if !ok {
		return
	}
	canvas, err := model.GetYucoreCanvasById(id, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildYucoreCanvasResponse(canvas))
}

func UpdateYucoreCanvas(c *gin.Context) {
	id, ok := parseYucoreCanvasId(c)
	if !ok {
		return
	}
	var req yucoreCanvasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	canvas, err := model.GetYucoreCanvasById(id, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	nextCanvas, err := buildYucoreCanvasFromRequest(req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	canvas.Title = nextCanvas.Title
	canvas.Description = nextCanvas.Description
	canvas.Module = nextCanvas.Module
	canvas.Snapshot = nextCanvas.Snapshot
	canvas.Viewport = nextCanvas.Viewport
	if err := model.UpdateYucoreCanvas(canvas, !req.Autosave); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildYucoreCanvasResponse(canvas))
}

func DeleteYucoreCanvas(c *gin.Context) {
	id, ok := parseYucoreCanvasId(c)
	if !ok {
		return
	}
	if err := model.DeleteYucoreCanvasById(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ListYucoreCanvasVersions(c *gin.Context) {
	id, ok := parseYucoreCanvasId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if _, err := model.GetYucoreCanvasById(id, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	versions, total, err := model.ListYucoreCanvasVersions(id, userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]yucoreCanvasVersionResponse, 0, len(versions))
	for _, version := range versions {
		items = append(items, buildYucoreCanvasVersionResponse(version))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func ListYucoreCanvasAgentRuns(c *gin.Context) {
	id, ok := parseYucoreCanvasId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if _, err := model.GetYucoreCanvasById(id, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	runs, err := model.ListYucoreCanvasAgentRuns(id, userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountYucoreCanvasAgentRuns(id, userId)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildYucoreCanvasAgentRunResponses(runs))
	common.ApiSuccess(c, pageInfo)
}

func CreateYucoreCanvasAgentRun(c *gin.Context) {
	id, ok := parseYucoreCanvasId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if _, err := model.GetYucoreCanvasById(id, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	var req yucoreCanvasAgentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	run, err := buildYucoreCanvasAgentRunFromRequest(req, id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.CreateYucoreCanvasAgentRun(run); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildYucoreCanvasAgentRunResponse(run))
}

func ExecuteYucoreCanvasAgentRun(c *gin.Context) {
	id, ok := parseYucoreCanvasId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if _, err := model.GetYucoreCanvasById(id, userId); err != nil {
		common.ApiError(c, err)
		return
	}

	var req yucoreCanvasAgentExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	promptNodeId, err := normalizeYucoreCanvasNodeId(req.AgentPromptNodeId, "agent prompt node id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	taskNodeId, err := normalizeYucoreCanvasNodeId(req.AgentTaskNodeId, "agent task node id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	req.AgentPromptNodeId = promptNodeId
	req.AgentTaskNodeId = taskNodeId

	identity, err := buildYucoreCanvasIdentity(userId, c.GetString("username"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	task, err := buildYucoreMediaTaskFromRequest(buildYucoreCanvasAgentExecuteMediaRequest(req), userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := resolveYucoreMediaTaskRequest(task); err != nil {
		common.ApiError(c, err)
		return
	}
	task.TaskId = model.GenerateYucoreMediaTaskID()
	runId := model.GenerateYucoreCanvasAgentRunID()

	task.Metadata, err = mergeYucoreCanvasObjectJSON(task.Metadata, map[string]any{
		"surface":                    "yucore-studio",
		"canvas_id":                  id,
		"agent_run_id":               runId,
		"agent_prompt_node_id":       promptNodeId,
		"agent_task_node_id":         taskNodeId,
		"agent_mode":                 strings.TrimSpace(req.Mode),
		"canvas_identity_session":    identity.IdentitySession,
		"canvas_identity_expires_at": identity.ExpiresAt,
		"agent_execute_endpoint":     "/api/yucore/canvas/:id/agent-runs/execute",
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	actions, err := buildYucoreCanvasAgentExecuteActions(req, task, identity)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	run, err := buildYucoreCanvasAgentRunFromRequest(yucoreCanvasAgentRunRequest{
		Mode:         req.Mode,
		Prompt:       req.Prompt,
		Status:       model.YucoreCanvasAgentRunStatusRunning,
		Summary:      fmt.Sprintf("Queued %s and is waiting for provider result.", task.TaskId),
		Actions:      actions,
		ResultTaskId: task.TaskId,
	}, id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	run.RunId = runId

	upstreamHeaders := yucoreMediaUAGProxyHeadersFromRequest(c)
	if strings.TrimSpace(upstreamHeaders["X-YuCore-Canvas-Identity"]) == "" {
		upstreamHeaders["X-YuCore-Canvas-Identity"] = identity.IdentityToken
	}
	if strings.TrimSpace(upstreamHeaders["X-YuCore-Canvas-Session"]) == "" {
		upstreamHeaders["X-YuCore-Canvas-Session"] = identity.IdentitySession
	}
	if err := model.CreateYucoreCanvasAgentExecution(run, task, upstreamHeaders); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, yucoreCanvasAgentExecuteResponse{
		Run:      buildYucoreCanvasAgentRunResponse(run),
		Task:     buildYucoreMediaTaskResponse(task),
		Identity: identity,
	})
}

func UpdateYucoreCanvasAgentRun(c *gin.Context) {
	id, ok := parseYucoreCanvasId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if _, err := model.GetYucoreCanvasById(id, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	runId := strings.TrimSpace(c.Param("run_id"))
	if runId == "" {
		common.ApiErrorMsg(c, "invalid agent run id")
		return
	}
	run, err := model.GetYucoreCanvasAgentRunByRunId(runId, id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req yucoreCanvasAgentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	nextRun, err := buildYucoreCanvasAgentRunFromRequest(req, id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	run.Mode = nextRun.Mode
	run.Prompt = nextRun.Prompt
	run.Status = nextRun.Status
	run.Summary = nextRun.Summary
	run.Actions = nextRun.Actions
	run.ResultTaskId = nextRun.ResultTaskId
	if err := model.UpdateYucoreCanvasAgentRun(run); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildYucoreCanvasAgentRunResponse(run))
}
