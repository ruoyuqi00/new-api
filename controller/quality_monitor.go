package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func qualityMonitorAdmin(c *gin.Context) bool {
	if c.GetInt("id") < 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Authentication required", "data": nil})
		return false
	}
	if c.GetInt("role") < common.RoleAdminUser {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Administrator access required", "data": nil})
		return false
	}
	return true
}

func qualityMonitorAllowedGroups(c *gin.Context) (map[string]string, error) {
	if c.GetInt("id") < 1 {
		return nil, errors.New("authentication required")
	}
	if c.GetInt("role") >= common.RoleAdminUser {
		return nil, nil
	}
	var user model.User
	if err := model.DB.Select("id", "group").First(&user, c.GetInt("id")).Error; err != nil {
		return nil, err
	}
	return service.GetUserUsableGroups(user.Group), nil
}

func qualityMonitorFailure(c *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, gorm.ErrRecordNotFound) {
		status = http.StatusNotFound
	}
	if errors.Is(err, service.ErrQualityMonitorBusy) {
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{"success": false, "message": err.Error(), "data": nil})
}

func GetQualityMonitorOptions(c *gin.Context) {
	groups, err := qualityMonitorAllowedGroups(c)
	if err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	options, err := service.GetQualityMonitorOptions(groups)
	if err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": options})
}

func GetQualityMonitorPlans(c *gin.Context) {
	if !qualityMonitorAdmin(c) {
		return
	}
	plans, err := model.ListQualityMonitorPlans()
	if err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": plans})
}

func CreateQualityMonitorPlan(c *gin.Context) {
	if !qualityMonitorAdmin(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	var plan model.QualityMonitorPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		qualityMonitorFailure(c, errors.New("invalid plan configuration"))
		return
	}
	if err := service.ValidateQualityMonitorPlan(&plan); err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	if err := model.SaveQualityMonitorPlan(&plan, true); err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": plan})
}

func UpdateQualityMonitorPlan(c *gin.Context) {
	if !qualityMonitorAdmin(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		qualityMonitorFailure(c, errors.New("invalid plan id"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	var plan model.QualityMonitorPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		qualityMonitorFailure(c, errors.New("invalid plan configuration"))
		return
	}
	plan.ID = id
	existing, err := model.GetQualityMonitorPlan(id)
	if err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	if err := service.ValidateQualityMonitorPlan(&plan, existing.Groups); err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	if err := model.SaveQualityMonitorPlan(&plan, false); err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": plan})
}

func DeleteQualityMonitorPlan(c *gin.Context) {
	if !qualityMonitorAdmin(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		qualityMonitorFailure(c, errors.New("invalid plan id"))
		return
	}
	if _, err := model.GetQualityMonitorPlan(id); err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	if err := model.DeleteQualityMonitorPlan(id); err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": nil})
}

func RunQualityMonitorPlan(c *gin.Context) {
	if !qualityMonitorAdmin(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		qualityMonitorFailure(c, errors.New("invalid plan id"))
		return
	}
	task, err := service.StartQualityMonitorRun(id)
	if err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"task_id": task.TaskID}})
}

func GetQualityMonitorResults(c *gin.Context) {
	groups, err := qualityMonitorAllowedGroups(c)
	if err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	planID, _ := strconv.Atoi(c.Query("plan_id"))
	if page < 1 || page > 1000000 || pageSize < 1 || pageSize > 100 || planID < 0 {
		qualityMonitorFailure(c, errors.New("invalid result pagination or filter"))
		return
	}
	filter := model.QualityMonitorResultFilter{Admin: c.GetInt("role") >= common.RoleAdminUser, Group: c.Query("group"), Model: c.Query("model"), PlanID: planID, Page: page, PageSize: pageSize}
	for group := range groups {
		filter.AllowedGroups = append(filter.AllowedGroups, group)
	}
	items, total, err := model.ListQualityMonitorResults(filter)
	if err != nil {
		qualityMonitorFailure(c, err)
		return
	}
	// An explicit public view prevents new internal fields from becoming visible.
	if !filter.Admin {
		public := make([]gin.H, 0, len(items))
		for _, r := range items {
			public = append(public, gin.H{"id": r.ID, "plan_id": r.PlanID, "plan_name": r.PlanName, "group": r.Group, "model": r.Model, "question_id": r.QuestionID, "prompt": r.Prompt, "expected_answer": r.ExpectedAnswer, "answer": r.Answer, "status": r.Status, "duration_ms": r.DurationMS, "created_at": r.CreatedAt, "answer_truncated": r.AnswerTruncated, "reasoning_effort": r.ReasoningEffort})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"items": public, "total": total}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"items": items, "total": total}})
}

func probeQualityMonitorChannel(ctx context.Context, channel *model.Channel, plan model.QualityMonitorPlan, question model.QualityMonitorQuestion) (service.QualityMonitorProbeResult, error) {
	input, err := common.Marshal(question.Prompt)
	if err != nil {
		return service.QualityMonitorProbeResult{}, err
	}
	budget := uint(plan.MaxOutputTokens)
	stream := false
	request := &dto.OpenAIResponsesRequest{Model: plan.Models[0], Input: input, Reasoning: &dto.Reasoning{Effort: plan.ReasoningEffort}, MaxOutputTokens: &budget, Stream: &stream, Store: []byte("false")}
	result := testChannel(ctx, channel, 0, plan.Models[0], string(constant.EndpointTypeOpenAIResponse), false, "", channelTestOptions{Request: request, Group: plan.Groups[0]})
	if result.localErr != nil {
		if result.newAPIError != nil {
			return service.QualityMonitorProbeResult{}, result.newAPIError
		}
		return service.QualityMonitorProbeResult{}, result.localErr
	}
	return service.QualityMonitorProbeResult{Answer: result.responseContent, AnswerTruncated: result.responseTruncated, ActualResponseModel: result.actualResponseModel, EstimatedQuota: result.estimatedQuota}, nil
}

type qualityMonitorHandler struct{}

func (qualityMonitorHandler) Type() string { return model.SystemTaskTypeQualityMonitor }
func (qualityMonitorHandler) Enabled() bool {
	now := common.GetTimestamp()
	if service.HasDueQualityMonitorPlans(now) {
		return true
	}
	var count int64
	return model.DB.Model(&model.QualityMonitorResult{}).Where("created_at < ?", now-30*86400).Limit(1).Count(&count).Error == nil && count > 0
}
func (qualityMonitorHandler) Interval() time.Duration { return 0 }
func (qualityMonitorHandler) NewPayload() any         { return nil }
func (qualityMonitorHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	err := service.RunQualityMonitorTask(ctx, task, runnerID, probeQualityMonitorChannel)
	status := model.SystemTaskStatusSucceeded
	if err != nil {
		status = model.SystemTaskStatusFailed
	}
	finishSystemTaskHandler(task, runnerID, status, nil, err)
}
