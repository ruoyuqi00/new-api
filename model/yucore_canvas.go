package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const DefaultYucoreCanvasModule = "Infinite canvas"

const (
	YucoreCanvasAgentRunStatusQueued    = "queued"
	YucoreCanvasAgentRunStatusRunning   = "running"
	YucoreCanvasAgentRunStatusCompleted = "completed"
	YucoreCanvasAgentRunStatusFailed    = "failed"
)

type YucoreCanvas struct {
	Id          int            `json:"id" gorm:"primary_key"`
	UserId      int            `json:"user_id" gorm:"index"`
	Title       string         `json:"title" gorm:"type:varchar(128);not null"`
	Description string         `json:"description" gorm:"type:varchar(512);default:''"`
	Module      string         `json:"module" gorm:"type:varchar(64);index;default:'Infinite canvas'"`
	Snapshot    string         `json:"snapshot" gorm:"type:text"`
	Viewport    string         `json:"viewport" gorm:"type:text"`
	Revision    int            `json:"revision" gorm:"default:1"`
	CreatedTime int64          `json:"created_time" gorm:"bigint;index"`
	UpdatedTime int64          `json:"updated_time" gorm:"bigint;index"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type YucoreCanvasVersion struct {
	Id          int    `json:"id" gorm:"primary_key"`
	CanvasId    int    `json:"canvas_id" gorm:"index"`
	UserId      int    `json:"user_id" gorm:"index"`
	Revision    int    `json:"revision" gorm:"index"`
	Title       string `json:"title" gorm:"type:varchar(128);not null"`
	Module      string `json:"module" gorm:"type:varchar(64);index;default:'Infinite canvas'"`
	Snapshot    string `json:"snapshot" gorm:"type:text"`
	Viewport    string `json:"viewport" gorm:"type:text"`
	CreatedTime int64  `json:"created_time" gorm:"bigint;index"`
}

type YucoreCanvasAgentRun struct {
	Id           int            `json:"id" gorm:"primary_key"`
	RunId        string         `json:"run_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId       int            `json:"user_id" gorm:"index"`
	CanvasId     int            `json:"canvas_id" gorm:"index"`
	Mode         string         `json:"mode" gorm:"type:varchar(24);default:'site'"`
	Prompt       string         `json:"prompt" gorm:"type:text"`
	Status       string         `json:"status" gorm:"type:varchar(24);index;default:'queued'"`
	Summary      string         `json:"summary" gorm:"type:varchar(512);default:''"`
	Actions      string         `json:"actions" gorm:"type:text"`
	ResultTaskId string         `json:"result_task_id" gorm:"type:varchar(64);index;default:''"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint;index"`
	UpdatedTime  int64          `json:"updated_time" gorm:"bigint;index"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func GenerateYucoreCanvasAgentRunID() string {
	key, err := common.GenerateRandomCharsKey(16)
	if err != nil || key == "" {
		return "agent_" + strings.ToLower(common.GetUUID()[:12])
	}
	return "agent_" + key
}

func normalizeYucoreCanvas(canvas *YucoreCanvas) {
	canvas.Title = strings.TrimSpace(canvas.Title)
	if canvas.Title == "" {
		canvas.Title = "Untitled canvas"
	}
	canvas.Description = strings.TrimSpace(canvas.Description)
	canvas.Module = strings.TrimSpace(canvas.Module)
	if canvas.Module == "" {
		canvas.Module = DefaultYucoreCanvasModule
	}
	if canvas.Snapshot == "" {
		canvas.Snapshot = `{"nodes":[],"edges":[]}`
	}
	if canvas.Viewport == "" {
		canvas.Viewport = `{}`
	}
	if canvas.Revision <= 0 {
		canvas.Revision = 1
	}
}

func normalizeYucoreCanvasAgentRun(run *YucoreCanvasAgentRun) {
	run.RunId = strings.TrimSpace(run.RunId)
	if run.RunId == "" {
		run.RunId = GenerateYucoreCanvasAgentRunID()
	}
	run.Mode = strings.TrimSpace(run.Mode)
	if run.Mode == "" {
		run.Mode = "site"
	}
	run.Prompt = strings.TrimSpace(run.Prompt)
	run.Status = strings.TrimSpace(run.Status)
	if run.Status == "" {
		run.Status = YucoreCanvasAgentRunStatusQueued
	}
	run.Summary = strings.TrimSpace(run.Summary)
	run.ResultTaskId = strings.TrimSpace(run.ResultTaskId)
	if run.Actions == "" {
		run.Actions = `[]`
	}
}

func (canvas *YucoreCanvas) toVersion() YucoreCanvasVersion {
	return YucoreCanvasVersion{
		CanvasId:    canvas.Id,
		UserId:      canvas.UserId,
		Revision:    canvas.Revision,
		Title:       canvas.Title,
		Module:      canvas.Module,
		Snapshot:    canvas.Snapshot,
		Viewport:    canvas.Viewport,
		CreatedTime: canvas.UpdatedTime,
	}
}

func CreateYucoreCanvas(canvas *YucoreCanvas) error {
	normalizeYucoreCanvas(canvas)
	now := common.GetTimestamp()
	canvas.CreatedTime = now
	canvas.UpdatedTime = now
	canvas.Revision = 1

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(canvas).Error; err != nil {
			return err
		}
		version := canvas.toVersion()
		return tx.Create(&version).Error
	})
}

func CreateYucoreCanvasAgentRun(run *YucoreCanvasAgentRun) error {
	normalizeYucoreCanvasAgentRun(run)
	now := common.GetTimestamp()
	run.CreatedTime = now
	run.UpdatedTime = now
	return DB.Create(run).Error
}

func CreateYucoreCanvasAgentExecution(run *YucoreCanvasAgentRun, task *YucoreMediaTask, upstreamHeaders YucoreMediaUAGProxyHeaders) error {
	normalizeYucoreCanvasAgentRun(run)
	normalizeYucoreMediaTask(task)
	adapter, err := resolveYucoreMediaTaskAdapter(task)
	if err != nil {
		return err
	}
	task.Metadata = mergeYucoreMediaMetadata(task.Metadata, map[string]any{
		"adapter":             adapter,
		"adapter_configured":  adapter != YucoreMediaAdapterMock,
		"adapter_created_at":  common.GetTimestamp(),
		"real_assets_enabled": adapter != YucoreMediaAdapterMock,
	})

	now := common.GetTimestamp()
	task.TaskId = strings.TrimSpace(task.TaskId)
	if task.TaskId == "" {
		task.TaskId = GenerateYucoreMediaTaskID()
	}
	task.CreatedTime = now
	task.UpdatedTime = now
	task.Status = YucoreMediaTaskStatusProcessing
	task.Progress = 8

	if run.ResultTaskId == "" {
		run.ResultTaskId = task.TaskId
	}
	run.CreatedTime = now
	run.UpdatedTime = now

	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		return tx.Create(task).Error
	}); err != nil {
		return err
	}

	if isYucoreMediaRunnableAdapter(adapter) {
		go RunYucoreMediaTaskWithHeaders(task.Id, cloneYucoreMediaUAGProxyHeaders(upstreamHeaders))
	}
	return nil
}

func GetYucoreCanvasAgentRunByRunId(runId string, canvasId int, userId int) (*YucoreCanvasAgentRun, error) {
	var run YucoreCanvasAgentRun
	err := DB.Where("run_id = ? and canvas_id = ? and user_id = ?", runId, canvasId, userId).First(&run).Error
	return &run, err
}

func UpdateYucoreCanvasAgentRun(run *YucoreCanvasAgentRun) error {
	normalizeYucoreCanvasAgentRun(run)
	run.UpdatedTime = common.GetTimestamp()
	return DB.Model(run).
		Select("mode", "prompt", "status", "summary", "actions", "result_task_id", "updated_time").
		Updates(run).Error
}

func CountYucoreCanvasAgentRuns(canvasId int, userId int) (int64, error) {
	var total int64
	err := DB.Model(&YucoreCanvasAgentRun{}).
		Where("canvas_id = ? and user_id = ?", canvasId, userId).
		Count(&total).Error
	return total, err
}

func ListYucoreCanvasAgentRuns(canvasId int, userId int, startIdx int, num int) ([]*YucoreCanvasAgentRun, error) {
	var runs []*YucoreCanvasAgentRun
	err := DB.Where("canvas_id = ? and user_id = ?", canvasId, userId).
		Order("updated_time desc, id desc").
		Limit(num).
		Offset(startIdx).
		Find(&runs).Error
	return runs, err
}

func CountYucoreCanvases(userId int) (int64, error) {
	var total int64
	err := DB.Model(&YucoreCanvas{}).Where("user_id = ?", userId).Count(&total).Error
	return total, err
}

func ListYucoreCanvases(userId int, startIdx int, num int) ([]*YucoreCanvas, error) {
	var canvases []*YucoreCanvas
	err := DB.Where("user_id = ?", userId).
		Order("updated_time desc, id desc").
		Limit(num).
		Offset(startIdx).
		Find(&canvases).Error
	return canvases, err
}

func GetYucoreCanvasById(id int, userId int) (*YucoreCanvas, error) {
	var canvas YucoreCanvas
	err := DB.Where("id = ? and user_id = ?", id, userId).First(&canvas).Error
	return &canvas, err
}

func UpdateYucoreCanvas(canvas *YucoreCanvas, createVersion bool) error {
	normalizeYucoreCanvas(canvas)
	canvas.UpdatedTime = common.GetTimestamp()
	canvas.Revision++

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(canvas).
			Select("title", "description", "module", "snapshot", "viewport", "revision", "updated_time").
			Updates(canvas).Error; err != nil {
			return err
		}
		if !createVersion {
			return nil
		}
		version := canvas.toVersion()
		return tx.Create(&version).Error
	})
}

func yucoreCanvasMediaTaskAssetLabel(task *YucoreMediaTask) string {
	assets := YucoreMediaTaskAssets(task)
	count := len(assets)
	if count == 1 {
		return "1 asset"
	}
	return fmt.Sprintf("%d assets", count)
}

func buildYucoreCanvasTaskStatus(task *YucoreMediaTask) string {
	if task == nil {
		return ""
	}
	if task.Status == YucoreMediaTaskStatusCompleted {
		return fmt.Sprintf("%s / completed / %s", task.TaskId, yucoreCanvasMediaTaskAssetLabel(task))
	}
	if task.Status == YucoreMediaTaskStatusCanceled {
		return fmt.Sprintf("%s / canceled", task.TaskId)
	}
	message := strings.TrimSpace(task.Error)
	if message != "" {
		if len(message) > 140 {
			message = message[:137] + "..."
		}
		return fmt.Sprintf("%s / failed / %s", task.TaskId, message)
	}
	return fmt.Sprintf("%s / failed", task.TaskId)
}

func buildYucoreCanvasAgentRunSummary(task *YucoreMediaTask) string {
	if task == nil {
		return ""
	}
	if task.Status == YucoreMediaTaskStatusCompleted {
		return fmt.Sprintf("Media task %s completed with %s.", task.TaskId, yucoreCanvasMediaTaskAssetLabel(task))
	}
	if task.Status == YucoreMediaTaskStatusCanceled {
		return fmt.Sprintf("Media task %s was canceled.", task.TaskId)
	}
	message := strings.TrimSpace(task.Error)
	if message != "" {
		if len(message) > 140 {
			message = message[:137] + "..."
		}
		return fmt.Sprintf("Media task %s failed: %s", task.TaskId, message)
	}
	return fmt.Sprintf("Media task %s failed.", task.TaskId)
}

func yucoreCanvasSetMapValue(target map[string]any, key string, value any) bool {
	if reflect.DeepEqual(target[key], value) {
		return false
	}
	target[key] = value
	return true
}

func yucoreCanvasDeleteMapValue(target map[string]any, key string) bool {
	if _, ok := target[key]; !ok {
		return false
	}
	delete(target, key)
	return true
}

func buildYucoreCanvasAgentRunActions(actions string, task *YucoreMediaTask) string {
	if task == nil {
		return actions
	}
	var rows []map[string]any
	if strings.TrimSpace(actions) != "" {
		_ = json.Unmarshal([]byte(actions), &rows)
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	resultStatus := YucoreCanvasAgentRunStatusFailed
	if task.Status == YucoreMediaTaskStatusCompleted {
		resultStatus = YucoreCanvasAgentRunStatusCompleted
	}
	assets := YucoreMediaTaskAssets(task)
	resultAssets := make([]map[string]any, 0, len(assets))
	for _, asset := range assets {
		resultAssets = append(resultAssets, map[string]any{
			"id":        asset.Id,
			"kind":      asset.Kind,
			"url":       asset.Url,
			"thumb_url": asset.ThumbUrl,
		})
	}
	generationAction := map[string]any{
		"tool":    "canvas_run_generation",
		"status":  resultStatus,
		"task_id": task.TaskId,
		"assets":  resultAssets,
	}
	if strings.TrimSpace(task.Error) != "" && resultStatus == YucoreCanvasAgentRunStatusFailed {
		generationAction["error"] = task.Error
	}
	patchedGeneration := false
	hasApplyResult := false
	for index, row := range rows {
		if row == nil {
			continue
		}
		tool := yucoreMediaStringValue(row["tool"])
		if tool == "canvas_apply_result" && yucoreMediaStringValue(row["task_id"]) == task.TaskId {
			hasApplyResult = true
		}
		if tool == "canvas_run_generation" || (tool == "" && yucoreMediaStringValue(row["task_id"]) == task.TaskId) {
			for key, value := range generationAction {
				row[key] = value
			}
			rows[index] = row
			patchedGeneration = true
		}
	}
	if !patchedGeneration {
		rows = append(rows, generationAction)
	}
	if task.Status == YucoreMediaTaskStatusCompleted && len(resultAssets) > 0 && !hasApplyResult {
		assetIDs := make([]string, 0, len(resultAssets))
		for _, asset := range resultAssets {
			if id := yucoreMediaStringValue(asset["id"]); id != "" {
				assetIDs = append(assetIDs, id)
			}
		}
		rows = append(rows, map[string]any{
			"tool":      "canvas_apply_result",
			"status":    YucoreCanvasAgentRunStatusCompleted,
			"task_id":   task.TaskId,
			"asset_ids": assetIDs,
		})
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return actions
	}
	return string(raw)
}

func applyYucoreCanvasMediaTaskSnapshotBackflow(snapshot string, task *YucoreMediaTask, promptNodeId string, taskNodeId string) (string, bool, error) {
	if task == nil || strings.TrimSpace(taskNodeId) == "" {
		return snapshot, false, nil
	}
	raw := strings.TrimSpace(snapshot)
	if raw == "" {
		raw = `{"nodes":[],"edges":[]}`
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return snapshot, false, err
	}
	nodes, _ := root["nodes"].([]any)
	if len(nodes) == 0 {
		return snapshot, false, nil
	}
	assets := YucoreMediaTaskAssets(task)
	var firstAsset *YucoreMediaAsset
	if len(assets) > 0 {
		firstAsset = &assets[0]
	}
	changed := false
	for _, item := range nodes {
		node, ok := item.(map[string]any)
		if !ok {
			continue
		}
		nodeID := yucoreMediaStringValue(node["id"])
		if nodeID != taskNodeId && (promptNodeId == "" || nodeID != promptNodeId) {
			continue
		}
		data, _ := node["data"].(map[string]any)
		if data == nil {
			data = map[string]any{}
		}
		if nodeID == taskNodeId {
			changed = yucoreCanvasSetMapValue(data, "kind", task.Kind) || changed
			if task.Status == YucoreMediaTaskStatusCompleted && firstAsset != nil {
				if strings.TrimSpace(firstAsset.Label) != "" {
					changed = yucoreCanvasSetMapValue(data, "label", firstAsset.Label) || changed
				} else if yucoreMediaStringValue(data["label"]) == "" {
					changed = yucoreCanvasSetMapValue(data, "label", "Generation result") || changed
				}
			}
			changed = yucoreCanvasSetMapValue(data, "sublabel", task.ModelId) || changed
			changed = yucoreCanvasSetMapValue(data, "prompt", task.Prompt) || changed
			changed = yucoreCanvasSetMapValue(data, "status", buildYucoreCanvasTaskStatus(task)) || changed
			changed = yucoreCanvasSetMapValue(data, "resultTaskId", task.TaskId) || changed
			if strings.TrimSpace(task.Error) != "" {
				changed = yucoreCanvasSetMapValue(data, "error", task.Error) || changed
			} else {
				changed = yucoreCanvasDeleteMapValue(data, "error") || changed
			}
			if task.Status == YucoreMediaTaskStatusCompleted && firstAsset != nil {
				assetURL := firstAsset.Url
				if strings.TrimSpace(assetURL) == "" {
					assetURL = firstAsset.CachedUrl
				}
				if strings.TrimSpace(assetURL) == "" {
					assetURL = firstAsset.SourceUrl
				}
				thumbURL := firstAsset.ThumbUrl
				if strings.TrimSpace(thumbURL) == "" {
					thumbURL = firstAsset.CachedUrl
				}
				if strings.TrimSpace(thumbURL) == "" {
					thumbURL = firstAsset.SourceUrl
				}
				changed = yucoreCanvasSetMapValue(data, "assetUrl", assetURL) || changed
				changed = yucoreCanvasSetMapValue(data, "thumbUrl", thumbURL) || changed
			}
			style, _ := node["style"].(map[string]any)
			if style == nil {
				style = map[string]any{}
			}
			changed = yucoreCanvasSetMapValue(style, "width", float64(230)) || changed
			if task.Status == YucoreMediaTaskStatusCompleted {
				changed = yucoreCanvasSetMapValue(style, "border", "1px solid rgb(103 232 249 / 0.32)") || changed
				if firstAsset != nil {
					changed = yucoreCanvasSetMapValue(style, "padding", float64(0)) || changed
				}
			} else {
				changed = yucoreCanvasSetMapValue(style, "border", "1px solid rgb(251 113 133 / 0.32)") || changed
				if _, ok := style["padding"]; !ok {
					changed = yucoreCanvasSetMapValue(style, "padding", float64(14)) || changed
				}
			}
			changed = yucoreCanvasSetMapValue(style, "boxShadow", "0 22px 70px rgb(0 0 0 / 0.36)") || changed
			node["style"] = style
		}
		if promptNodeId != "" && nodeID == promptNodeId {
			changed = yucoreCanvasSetMapValue(data, "status", fmt.Sprintf("linked %s / %s", task.TaskId, task.Status)) || changed
		}
		node["data"] = data
	}
	if !changed {
		return snapshot, false, nil
	}
	next, err := json.Marshal(root)
	if err != nil {
		return snapshot, false, err
	}
	return string(next), true, nil
}

func ApplyYucoreCanvasAgentMediaBackflow(task *YucoreMediaTask) error {
	if task == nil {
		return nil
	}
	if task.Status != YucoreMediaTaskStatusCompleted && task.Status != YucoreMediaTaskStatusFailed && task.Status != YucoreMediaTaskStatusCanceled {
		return nil
	}
	metadata := yucoreMediaMetadataMap(task.Metadata)
	canvasId := yucoreMediaIntValue(metadata["canvas_id"])
	agentRunId := yucoreMediaFirstString(metadata, "agent_run_id")
	promptNodeId := yucoreMediaFirstString(metadata, "agent_prompt_node_id")
	taskNodeId := yucoreMediaFirstString(metadata, "agent_task_node_id")
	if canvasId <= 0 && agentRunId == "" && taskNodeId == "" {
		return nil
	}
	userId := task.UserId
	now := common.GetTimestamp()
	if canvasId > 0 && agentRunId != "" {
		var run YucoreCanvasAgentRun
		if err := DB.Where("run_id = ? and canvas_id = ? and user_id = ?", agentRunId, canvasId, userId).First(&run).Error; err == nil {
			if mode := yucoreMediaFirstString(metadata, "agent_mode"); mode != "" && run.Mode == "" {
				run.Mode = mode
			}
			if strings.TrimSpace(run.Prompt) == "" {
				run.Prompt = task.Prompt
			}
			run.Status = YucoreCanvasAgentRunStatusFailed
			if task.Status == YucoreMediaTaskStatusCompleted {
				run.Status = YucoreCanvasAgentRunStatusCompleted
			}
			run.Summary = buildYucoreCanvasAgentRunSummary(task)
			run.ResultTaskId = task.TaskId
			run.Actions = buildYucoreCanvasAgentRunActions(run.Actions, task)
			run.UpdatedTime = now
			if err := DB.Model(&run).
				Select("mode", "prompt", "status", "summary", "actions", "result_task_id", "updated_time").
				Updates(&run).Error; err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if canvasId > 0 && taskNodeId != "" {
		var canvas YucoreCanvas
		if err := DB.Where("id = ? and user_id = ?", canvasId, userId).First(&canvas).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		nextSnapshot, changed, err := applyYucoreCanvasMediaTaskSnapshotBackflow(canvas.Snapshot, task, promptNodeId, taskNodeId)
		if err != nil {
			return err
		}
		if changed {
			canvas.Snapshot = nextSnapshot
			return UpdateYucoreCanvas(&canvas, false)
		}
	}
	return nil
}

func DeleteYucoreCanvasById(id int, userId int) error {
	return DB.Where("id = ? and user_id = ?", id, userId).Delete(&YucoreCanvas{}).Error
}

func ListYucoreCanvasVersions(canvasId int, userId int, startIdx int, num int) ([]*YucoreCanvasVersion, int64, error) {
	var versions []*YucoreCanvasVersion
	query := DB.Model(&YucoreCanvasVersion{}).Where("canvas_id = ? and user_id = ?", canvasId, userId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("revision desc, id desc").Limit(num).Offset(startIdx).Find(&versions).Error
	return versions, total, err
}
