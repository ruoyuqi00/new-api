package relay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type TaskSubmitResult struct {
	UpstreamTaskID string
	TaskData       []byte
	Platform       constant.TaskPlatform
	Quota          int
	response       *taskResponseBuffer
	//PerCallPrice   types.PriceData
}

func (r *TaskSubmitResult) CommitResponse(c *gin.Context) error {
	if r == nil || r.response == nil {
		return errors.New("missing buffered task response")
	}
	for key, values := range r.response.Header() {
		c.Writer.Header()[key] = append([]string(nil), values...)
	}
	status := r.response.status
	if status == 0 {
		status = http.StatusOK
	}
	c.Writer.WriteHeader(status)
	_, err := c.Writer.Write(r.response.body.Bytes())
	return err
}

// ResolveOriginTask 处理基于已有任务的提交（remix / continuation）：
// 查找原始任务、从中提取模型名称、将渠道锁定到原始任务的渠道
// （通过 info.LockedChannel，重试时复用同一渠道并轮换 key），
// 以及提取 OtherRatios（时长、分辨率）。
// 该函数在控制器的重试循环之前调用一次，其结果通过 info 字段和上下文持久化。
func ResolveOriginTask(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	// 检测 remix action
	path := c.Request.URL.Path
	if strings.Contains(path, "/v1/videos/") && strings.HasSuffix(path, "/remix") {
		info.Action = constant.TaskActionRemix
	}

	// 提取 remix 任务的 video_id
	if info.Action == constant.TaskActionRemix {
		videoID := c.Param("video_id")
		if strings.TrimSpace(videoID) == "" {
			return service.TaskErrorWrapperLocal(fmt.Errorf("video_id is required"), "invalid_request", http.StatusBadRequest)
		}
		info.OriginTaskID = videoID
	}

	if info.OriginTaskID == "" {
		return nil
	}

	// 查找原始任务
	originTask, exist, err := model.GetByTaskId(info.UserId, info.OriginTaskID)
	if err != nil {
		return service.TaskErrorWrapper(err, "get_origin_task_failed", http.StatusInternalServerError)
	}
	if !exist {
		return service.TaskErrorWrapperLocal(errors.New("task_origin_not_exist"), "task_not_exist", http.StatusBadRequest)
	}

	// 从原始任务推导模型名称
	if info.OriginModelName == "" {
		if originTask.Properties.OriginModelName != "" {
			info.OriginModelName = originTask.Properties.OriginModelName
		} else if originTask.Properties.UpstreamModelName != "" {
			info.OriginModelName = originTask.Properties.UpstreamModelName
		} else {
			var taskData map[string]interface{}
			_ = common.Unmarshal(originTask.Data, &taskData)
			if m, ok := taskData["model"].(string); ok && m != "" {
				info.OriginModelName = m
			}
		}
	}

	// 锁定到原始任务的渠道（重试时复用同一渠道，轮换 key）
	ch, err := model.GetChannelById(originTask.ChannelId, true)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "channel_not_found", http.StatusBadRequest)
	}
	if ch.Status != common.ChannelStatusEnabled {
		return service.TaskErrorWrapperLocal(errors.New("the channel of the origin task is disabled"), "task_channel_disable", http.StatusBadRequest)
	}
	info.LockedChannel = ch

	if originTask.ChannelId != info.ChannelId {
		key, _, newAPIError := ch.GetNextEnabledKey()
		if newAPIError != nil {
			return service.TaskErrorWrapper(newAPIError, "channel_no_available_key", newAPIError.StatusCode)
		}
		common.SetContextKey(c, constant.ContextKeyChannelKey, key)
		common.SetContextKey(c, constant.ContextKeyChannelType, ch.Type)
		common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, ch.GetBaseURL())
		common.SetContextKey(c, constant.ContextKeyChannelId, originTask.ChannelId)

		info.ChannelBaseUrl = ch.GetBaseURL()
		info.ChannelId = originTask.ChannelId
		info.ChannelType = ch.Type
		info.ApiKey = key
	}

	// 提取 remix 参数（时长、分辨率 → OtherRatios）
	if info.Action == constant.TaskActionRemix {
		if originTask.PrivateData.BillingContext != nil {
			// 新的 remix 逻辑：直接从原始任务的 BillingContext 中提取 OtherRatios（如果存在）
			for s, f := range originTask.PrivateData.BillingContext.OtherRatios {
				info.PriceData.AddOtherRatio(s, f)
			}
		} else {
			// 旧的 remix 逻辑：直接从 task data 解析 seconds 和 size（如果存在）
			var taskData map[string]interface{}
			_ = common.Unmarshal(originTask.Data, &taskData)
			secondsStr, _ := taskData["seconds"].(string)
			seconds, _ := strconv.Atoi(secondsStr)
			if seconds <= 0 {
				seconds = 4
			}
			sizeStr, _ := taskData["size"].(string)
			if info.PriceData.OtherRatios == nil {
				info.PriceData.OtherRatios = map[string]float64{}
			}
			info.PriceData.OtherRatios["seconds"] = float64(seconds)
			info.PriceData.OtherRatios["size"] = 1
			if sizeStr == "1792x1024" || sizeStr == "1024x1792" {
				info.PriceData.OtherRatios["size"] = 1.666667
			}
		}
	}

	return nil
}

// RelayTaskSubmit 完成 task 提交的全部流程（每次尝试调用一次）：
// 刷新渠道元数据 → 确定 platform/adaptor → 验证请求 →
// 估算计费(EstimateBilling) → 计算价格 → 预扣费（仅首次）→
// 构建/发送/解析上游请求 → 提交后计费调整(AdjustBillingOnSubmit)。
// 控制器负责 defer Refund 和成功后 Settle。
func RelayTaskSubmit(c *gin.Context, info *relaycommon.RelayInfo) (*TaskSubmitResult, *dto.TaskError) {
	info.InitChannelMeta(c)

	// 1. 确定 platform → 创建适配器 → 验证请求
	platform := constant.TaskPlatform(c.GetString("platform"))
	if platform == "" {
		platform = GetTaskPlatform(c)
	}
	info.Platform = platform
	adaptor := GetTaskAdaptor(platform)
	if adaptor == nil {
		return nil, service.TaskErrorWrapperLocal(fmt.Errorf("invalid api platform: %s", platform), "invalid_api_platform", http.StatusBadRequest).WithSubmissionState(dto.TaskSubmissionNotSent)
	}
	adaptor.Init(info)
	if taskErr := adaptor.ValidateRequestAndSetAction(c, info); taskErr != nil {
		return nil, taskErr.WithSubmissionState(dto.TaskSubmissionNotSent)
	}

	// 2. 确定模型名称
	modelName := info.OriginModelName
	if modelName == "" {
		modelName = service.CoverTaskActionToModelName(platform, info.Action)
	}

	// 2.5 应用渠道的模型映射（与同步任务对齐）
	info.OriginModelName = modelName
	info.UpstreamModelName = modelName
	if err := helper.ModelMappedHelper(c, info, nil); err != nil {
		return nil, service.TaskErrorWrapperLocal(err, "model_mapping_failed", http.StatusBadRequest).WithSubmissionState(dto.TaskSubmissionNotSent)
	}

	// 3. 预生成公开 task ID（仅首次）
	if info.PublicTaskID == "" {
		info.PublicTaskID = model.GenerateTaskID()
	}

	// 4. 价格计算：基础模型价格
	if !info.TaskPricingResolved {
		info.OriginModelName = modelName
		priceData, err := helper.ModelPriceHelperPerCall(c, info)
		if err != nil {
			return nil, service.TaskErrorWrapper(err, "model_price_error", http.StatusBadRequest).WithSubmissionState(dto.TaskSubmissionNotSent)
		}
		info.PriceData = priceData
		resolveTaskPerCallBilling(info, modelName)

		// 5. 计费估算：让适配器根据用户请求提供 OtherRatios（时长、分辨率等）
		//    必须在 ModelPriceHelperPerCall 之后调用（它会重建 PriceData）。
		//    ResolveOriginTask 可能已在 remix 路径中预设了 OtherRatios，此处合并。
		if estimatedRatios := adaptor.EstimateBilling(c, info); len(estimatedRatios) > 0 {
			for k, v := range estimatedRatios {
				info.PriceData.AddOtherRatio(k, v)
			}
		}

		// 6. 将 OtherRatios 应用到基础额度
		if !info.TaskPerCallBilling {
			quota, clamp := applyTaskOtherRatiosQuotaChecked(info.PriceData.Quota, info.PriceData.OtherRatios)
			noteTaskQuotaClamp(info, clamp, "task_submit_other_ratios")
			info.PriceData.Quota = quota
		}
	}

	// 7. 预扣费（仅首次 — 重试时 info.Billing 已存在，跳过）
	if _, err := service.PersistTaskSubmissionIntent(info, platform); err != nil {
		return nil, service.TaskErrorWrapperLocal(errors.New("task submission state could not be persisted"), "task_persistence_failed", http.StatusInternalServerError).
			WithSubmissionState(dto.TaskSubmissionNotSent)
	}
	if info.Billing == nil && !info.PriceData.FreeModel {
		info.ForcePreConsume = true
		if apiErr := service.PreConsumeBilling(c, info.PriceData.Quota, info); apiErr != nil {
			return nil, service.TaskErrorFromAPIError(apiErr).WithSubmissionState(dto.TaskSubmissionNotSent)
		}
	}

	// 8. 构建请求体
	if _, err := service.PersistTaskSubmissionIntent(info, platform); err != nil {
		return nil, service.TaskErrorWrapperLocal(errors.New("task submission state could not be persisted"), "task_persistence_failed", http.StatusInternalServerError).
			WithSubmissionState(dto.TaskSubmissionNotSent)
	}
	requestBody, err := adaptor.BuildRequestBody(c, info)
	if err != nil {
		return nil, service.TaskErrorWrapper(err, "build_request_failed", http.StatusInternalServerError).WithSubmissionState(dto.TaskSubmissionNotSent)
	}

	// 9. 发送请求
	if err := service.MarkTaskSubmissionWritePossible(info); err != nil {
		return nil, service.TaskErrorWrapperLocal(errors.New("task submission state could not be persisted"), "task_persistence_failed", http.StatusInternalServerError).
			WithSubmissionState(dto.TaskSubmissionNotSent)
	}
	info.BeginRequestAttempt()
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		state := classifyTaskSubmissionState(info.RequestWritten, 0, false)
		if state == dto.TaskSubmissionNotSent {
			if persistErr := service.MarkTaskSubmissionRejected(info); persistErr != nil {
				return nil, service.TaskErrorWrapperLocal(errors.New("task submission state could not be persisted"), "task_persistence_failed", http.StatusInternalServerError).
					WithSubmissionState(dto.TaskSubmissionAmbiguous)
			}
		}
		return nil, service.TaskErrorWrapper(err, "do_request_failed", http.StatusInternalServerError).WithSubmissionState(state)
	}
	if resp == nil {
		state := classifyTaskSubmissionState(info.RequestWritten, 0, false)
		if state == dto.TaskSubmissionNotSent {
			if persistErr := service.MarkTaskSubmissionRejected(info); persistErr != nil {
				return nil, service.TaskErrorWrapperLocal(errors.New("task submission state could not be persisted"), "task_persistence_failed", http.StatusInternalServerError).
					WithSubmissionState(dto.TaskSubmissionAmbiguous)
			}
		}
		return nil, service.TaskErrorWrapper(errors.New("upstream returned no task submission response"), "invalid_response", http.StatusBadGateway).
			WithSubmissionState(state)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_ = resp.Body.Close()
		state := classifyTaskSubmissionState(info.RequestWritten, resp.StatusCode, false)
		if state == dto.TaskSubmissionRejected || state == dto.TaskSubmissionNotSent {
			if err := service.MarkTaskSubmissionRejected(info); err != nil {
				return nil, service.TaskErrorWrapperLocal(errors.New("task submission state could not be persisted"), "task_persistence_failed", http.StatusInternalServerError).
					WithSubmissionState(dto.TaskSubmissionAmbiguous)
			}
		}
		return nil, service.TaskErrorWrapper(fmt.Errorf("upstream rejected task submission with status %d", resp.StatusCode), "fail_to_fetch_task", resp.StatusCode).
			WithSubmissionState(state)
	}

	// 10. 返回 OtherRatios 给下游（header 必须在 DoResponse 写 body 之前设置）
	otherRatios := info.PriceData.OtherRatios
	if otherRatios == nil {
		otherRatios = map[string]float64{}
	}
	ratiosJSON, _ := common.Marshal(otherRatios)
	c.Header("X-New-Api-Other-Ratios", string(ratiosJSON))

	// 11. 解析响应
	upstreamTaskID, taskData, response, taskErr := doValidatedTaskResponse(c, adaptor, resp, info)
	if taskErr != nil {
		if taskErr.SubmissionState() == dto.TaskSubmissionRejected {
			if err := service.MarkTaskSubmissionRejected(info); err != nil {
				return nil, service.TaskErrorWrapperLocal(errors.New("task submission state could not be persisted"), "task_persistence_failed", http.StatusInternalServerError).
					WithSubmissionState(dto.TaskSubmissionAmbiguous)
			}
		}
		return nil, taskErr
	}

	// 11. 提交后计费调整：让适配器根据上游实际返回调整 OtherRatios
	finalQuota := info.PriceData.Quota
	if adjustedRatios := adaptor.AdjustBillingOnSubmit(info, taskData); !info.TaskPerCallBilling && len(adjustedRatios) > 0 {
		// 基于调整后的 ratios 重新计算 quota
		var clamp *common.QuotaClamp
		finalQuota, clamp = recalcQuotaFromRatiosChecked(info, adjustedRatios)
		noteTaskQuotaClamp(info, clamp, "task_submit_adjusted_ratios")
		info.PriceData.OtherRatios = adjustedRatios
		info.PriceData.Quota = finalQuota
	}

	return &TaskSubmitResult{
		UpstreamTaskID: upstreamTaskID,
		TaskData:       taskData,
		Platform:       platform,
		Quota:          finalQuota,
		response:       response,
	}, nil
}

type taskResponseBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *taskResponseBuffer) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *taskResponseBuffer) WriteHeader(statusCode int) {
	if w.status == 0 {
		w.status = statusCode
	}
}

func (w *taskResponseBuffer) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func doValidatedTaskResponse(c *gin.Context, adaptor channel.TaskAdaptor, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *taskResponseBuffer, *dto.TaskError) {
	buffer := &taskResponseBuffer{}
	bufferedContext, _ := gin.CreateTestContext(buffer)
	bufferedContext.Request = c.Request
	bufferedContext.Keys = c.Keys
	bufferedContext.Params = c.Params

	upstreamTaskID, taskData, taskErr := adaptor.DoResponse(bufferedContext, resp, info)
	if taskErr != nil {
		if taskErr.SubmissionState() == dto.TaskSubmissionRejected {
			return "", nil, nil, taskErr.WithSubmissionState(dto.TaskSubmissionRejected)
		}
		return "", nil, nil, service.TaskErrorWrapper(errors.New("upstream accepted task submission but returned an invalid response"), "invalid_response", http.StatusBadGateway).
			WithSubmissionState(dto.TaskSubmissionAccepted)
	}
	if upstreamTaskID == "" {
		return "", nil, nil, service.TaskErrorWrapper(errors.New("upstream accepted task submission without a task id"), "invalid_response", http.StatusBadGateway).
			WithSubmissionState(dto.TaskSubmissionAccepted)
	}
	return upstreamTaskID, taskData, buffer, nil
}

func classifyTaskSubmissionState(wroteRequest bool, statusCode int, acceptedResponse bool) dto.TaskSubmissionState {
	if acceptedResponse || statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return dto.TaskSubmissionAccepted
	}
	switch statusCode {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
		http.StatusConflict, http.StatusUnprocessableEntity, http.StatusTooManyRequests:
		return dto.TaskSubmissionRejected
	}
	if statusCode == http.StatusRequestTimeout {
		return dto.TaskSubmissionAmbiguous
	}
	if statusCode != 0 {
		return dto.TaskSubmissionAmbiguous
	}
	if wroteRequest {
		return dto.TaskSubmissionAmbiguous
	}
	return dto.TaskSubmissionNotSent
}

func resolveTaskPerCallBilling(info *relaycommon.RelayInfo, modelName string) {
	if info.TaskPricingResolved {
		return
	}
	info.TaskPerCallBilling = model.YucoreMediaModelUsesPerCallPricing(modelName)
	info.TaskPricingResolved = true
}

// recalcQuotaFromRatios 根据 adjustedRatios 重新计算 quota。
// 公式: baseQuota × ∏(ratio) — 其中 baseQuota 是不含 OtherRatios 的基础额度。
func recalcQuotaFromRatios(info *relaycommon.RelayInfo, ratios map[string]float64) int {
	quota, _ := recalcQuotaFromRatiosChecked(info, ratios)
	return quota
}

func recalcQuotaFromRatiosChecked(info *relaycommon.RelayInfo, ratios map[string]float64) (int, *common.QuotaClamp) {
	// 从 PriceData 获取不含 OtherRatios 的基础价格
	baseQuota := float64(info.PriceData.Quota)
	// 先除掉原有的 OtherRatios 恢复基础额度
	for _, ra := range info.PriceData.OtherRatios {
		if ra != 1.0 && ra > 0 {
			baseQuota /= ra
		}
	}
	// 应用新的 ratios
	result := baseQuota
	for _, ra := range ratios {
		if ra != 1.0 {
			result *= ra
		}
	}
	return common.QuotaFromFloatChecked(result)
}

func applyTaskOtherRatiosQuota(baseQuota int, ratios map[string]float64) int {
	quota, _ := applyTaskOtherRatiosQuotaChecked(baseQuota, ratios)
	return quota
}

func applyTaskOtherRatiosQuotaChecked(baseQuota int, ratios map[string]float64) (int, *common.QuotaClamp) {
	result := float64(baseQuota)
	for _, ra := range ratios {
		if ra != 1.0 {
			result *= ra
		}
	}
	return common.QuotaFromFloatChecked(result)
}

func noteTaskQuotaClamp(info *relaycommon.RelayInfo, clamp *common.QuotaClamp, op string) {
	if info == nil || info.TaskQuotaClamp != nil || clamp == nil {
		return
	}
	info.TaskQuotaClamp = clamp.WithOp(op)
}

var fetchRespBuilders = map[int]func(c *gin.Context) (respBody []byte, taskResp *dto.TaskError){
	relayconstant.RelayModeSunoFetchByID:  sunoFetchByIDRespBodyBuilder,
	relayconstant.RelayModeSunoFetch:      sunoFetchRespBodyBuilder,
	relayconstant.RelayModeVideoFetchByID: videoFetchByIDRespBodyBuilder,
}

func RelayTaskFetch(c *gin.Context, relayMode int) (taskResp *dto.TaskError) {
	respBuilder, ok := fetchRespBuilders[relayMode]
	if !ok {
		taskResp = service.TaskErrorWrapperLocal(errors.New("invalid_relay_mode"), "invalid_relay_mode", http.StatusBadRequest)
	}

	respBody, taskErr := respBuilder(c)
	if taskErr != nil {
		return taskErr
	}
	if len(respBody) == 0 {
		respBody = []byte("{\"code\":\"success\",\"data\":null}")
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	_, err := io.Copy(c.Writer, bytes.NewBuffer(respBody))
	if err != nil {
		taskResp = service.TaskErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
		return
	}
	return
}

func sunoFetchRespBodyBuilder(c *gin.Context) (respBody []byte, taskResp *dto.TaskError) {
	userId := c.GetInt("id")
	var condition = struct {
		IDs    []any  `json:"ids"`
		Action string `json:"action"`
	}{}
	err := c.BindJSON(&condition)
	if err != nil {
		taskResp = service.TaskErrorWrapper(err, "invalid_request", http.StatusBadRequest)
		return
	}
	var tasks []any
	if len(condition.IDs) > 0 {
		taskModels, err := model.GetByTaskIds(userId, condition.IDs)
		if err != nil {
			taskResp = service.TaskErrorWrapper(err, "get_tasks_failed", http.StatusInternalServerError)
			return
		}
		for _, task := range taskModels {
			tasks = append(tasks, TaskModel2UserDto(task))
		}
	} else {
		tasks = make([]any, 0)
	}
	respBody, err = common.Marshal(dto.TaskResponse[[]any]{
		Code: "success",
		Data: tasks,
	})
	return
}

func sunoFetchByIDRespBodyBuilder(c *gin.Context) (respBody []byte, taskResp *dto.TaskError) {
	taskId := c.Param("id")
	userId := c.GetInt("id")

	originTask, exist, err := model.GetByTaskId(userId, taskId)
	if err != nil {
		taskResp = service.TaskErrorWrapper(err, "get_task_failed", http.StatusInternalServerError)
		return
	}
	if !exist {
		taskResp = service.TaskErrorWrapperLocal(errors.New("task_not_exist"), "task_not_exist", http.StatusBadRequest)
		return
	}

	respBody, err = common.Marshal(dto.TaskResponse[any]{
		Code: "success",
		Data: TaskModel2UserDto(originTask),
	})
	return
}

func videoFetchByIDRespBodyBuilder(c *gin.Context) (respBody []byte, taskResp *dto.TaskError) {
	taskId := c.Param("task_id")
	if taskId == "" {
		taskId = c.GetString("task_id")
	}
	userId := c.GetInt("id")

	originTask, exist, err := model.GetByTaskId(userId, taskId)
	if err != nil {
		taskResp = service.TaskErrorWrapper(err, "get_task_failed", http.StatusInternalServerError)
		return
	}
	if !exist {
		taskResp = service.TaskErrorWrapperLocal(errors.New("task_not_exist"), "task_not_exist", http.StatusBadRequest)
		return
	}

	isOpenAIVideoAPI := strings.HasPrefix(c.Request.RequestURI, "/v1/videos/")

	// Gemini/Vertex 支持实时查询：用户 fetch 时直接从上游拉取最新状态
	if realtimeResp := tryRealtimeFetch(originTask, isOpenAIVideoAPI); len(realtimeResp) > 0 {
		respBody = realtimeResp
		return
	}

	// OpenAI Video API 格式: 走各 adaptor 的 ConvertToOpenAIVideo
	if isOpenAIVideoAPI {
		adaptor := GetTaskAdaptor(originTask.Platform)
		if adaptor == nil {
			taskResp = service.TaskErrorWrapperLocal(fmt.Errorf("invalid channel id: %d", originTask.ChannelId), "invalid_channel_id", http.StatusBadRequest)
			return
		}
		if converter, ok := adaptor.(channel.OpenAIVideoConverter); ok {
			openAIVideoData, err := converter.ConvertToOpenAIVideo(originTask)
			if err != nil {
				taskResp = service.TaskErrorWrapper(err, "convert_to_openai_video_failed", http.StatusInternalServerError)
				return
			}
			respBody = openAIVideoData
			return
		}
		taskResp = service.TaskErrorWrapperLocal(fmt.Errorf("not_implemented:%s", originTask.Platform), "not_implemented", http.StatusNotImplemented)
		return
	}

	// 通用 TaskDto 格式
	respBody, err = common.Marshal(dto.TaskResponse[any]{
		Code: "success",
		Data: TaskModel2UserDto(originTask),
	})
	if err != nil {
		taskResp = service.TaskErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
	}
	return
}

// tryRealtimeFetch 尝试从上游实时拉取 Gemini/Vertex 任务状态。
// 仅当渠道类型为 Gemini 或 Vertex 时触发；其他渠道或出错时返回 nil。
// 当非 OpenAI Video API 时，还会构建自定义格式的响应体。
func tryRealtimeFetch(task *model.Task, isOpenAIVideoAPI bool) []byte {
	channelModel, err := model.GetChannelById(task.ChannelId, true)
	if err != nil {
		return nil
	}
	if channelModel.Type != constant.ChannelTypeVertexAi && channelModel.Type != constant.ChannelTypeGemini {
		return nil
	}

	baseURL := constant.ChannelBaseURLs[channelModel.Type]
	if channelModel.GetBaseURL() != "" {
		baseURL = channelModel.GetBaseURL()
	}
	proxy := channelModel.GetSetting().Proxy
	adaptor := GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(channelModel.Type)))
	if adaptor == nil {
		return nil
	}

	resp, err := adaptor.FetchTask(baseURL, channelModel.Key, map[string]any{
		"task_id": task.GetUpstreamTaskID(),
		"action":  task.Action,
	}, proxy)
	if err != nil || resp == nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	ti, err := adaptor.ParseTaskResult(body)
	if err != nil || ti == nil {
		return nil
	}

	snap := task.Snapshot()

	// 将上游最新状态更新到 task
	if ti.Status != "" {
		task.Status = model.TaskStatus(ti.Status)
	}
	if ti.Progress != "" {
		task.Progress = ti.Progress
	}
	if strings.HasPrefix(ti.Url, "data:") {
		// data: URI — kept in Data, not ResultURL
	} else if ti.Url != "" {
		task.PrivateData.ResultURL = ti.Url
	} else if task.Status == model.TaskStatusSuccess {
		// No URL from adaptor — construct proxy URL using public task ID
		task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
	}

	if !snap.Equal(task.Snapshot()) {
		_, _ = task.UpdateWithStatus(snap.Status)
	}

	// OpenAI Video API 由调用者的 ConvertToOpenAIVideo 分支处理
	if isOpenAIVideoAPI {
		return nil
	}

	// 非 OpenAI Video API: 构建自定义格式响应
	format := detectVideoFormat(body)
	out := map[string]any{
		"error":    nil,
		"format":   format,
		"metadata": nil,
		"status":   mapTaskStatusToSimple(task.Status),
		"task_id":  task.TaskID,
		"url":      task.GetResultURL(),
	}
	respBody, _ := common.Marshal(dto.TaskResponse[any]{
		Code: "success",
		Data: out,
	})
	return respBody
}

// detectVideoFormat 从 Gemini/Vertex 原始响应中探测视频格式
func detectVideoFormat(rawBody []byte) string {
	var raw map[string]any
	if err := common.Unmarshal(rawBody, &raw); err != nil {
		return "mp4"
	}
	respObj, ok := raw["response"].(map[string]any)
	if !ok {
		return "mp4"
	}
	vids, ok := respObj["videos"].([]any)
	if !ok || len(vids) == 0 {
		return "mp4"
	}
	v0, ok := vids[0].(map[string]any)
	if !ok {
		return "mp4"
	}
	mt, ok := v0["mimeType"].(string)
	if !ok || mt == "" || strings.Contains(mt, "mp4") {
		return "mp4"
	}
	return mt
}

// mapTaskStatusToSimple 将内部 TaskStatus 映射为简化状态字符串
func mapTaskStatusToSimple(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusSuccess:
		return "succeeded"
	case model.TaskStatusFailure:
		return "failed"
	case model.TaskStatusQueued, model.TaskStatusSubmitted:
		return "queued"
	default:
		return "processing"
	}
}

func TaskModel2Dto(task *model.Task) *dto.TaskDto {
	return &dto.TaskDto{
		ID:         task.ID,
		CreatedAt:  task.CreatedAt,
		UpdatedAt:  task.UpdatedAt,
		TaskID:     task.TaskID,
		Platform:   string(task.Platform),
		UserId:     task.UserId,
		Group:      task.Group,
		ChannelId:  task.ChannelId,
		Quota:      task.Quota,
		Action:     task.Action,
		Status:     string(task.Status),
		FailReason: task.FailReason,
		ResultURL:  task.GetResultURL(),
		SubmitTime: task.SubmitTime,
		StartTime:  task.StartTime,
		FinishTime: task.FinishTime,
		Progress:   task.Progress,
		Properties: task.Properties,
		Username:   task.Username,
		Data:       task.Data,
	}
}

func TaskModel2UserDto(task *model.Task) *dto.TaskDto {
	taskDto := TaskModel2Dto(task)
	taskDto.Properties = model.Properties{
		Input:           task.Properties.Input,
		OriginModelName: task.Properties.OriginModelName,
	}

	originModel := task.Properties.OriginModelName
	upstreamModel := task.Properties.UpstreamModelName
	if originModel == "" || upstreamModel == "" || originModel == upstreamModel {
		return taskDto
	}

	var data map[string]json.RawMessage
	if err := common.Unmarshal(task.Data, &data); err != nil {
		return taskDto
	}
	publicModel, err := common.Marshal(originModel)
	if err != nil {
		return taskDto
	}

	changed := false
	if _, exists := data["model"]; exists {
		data["model"] = publicModel
		changed = true
	}
	if rawResponse, exists := data["response"]; exists {
		var response map[string]json.RawMessage
		if common.Unmarshal(rawResponse, &response) == nil {
			if _, exists := response["model"]; exists {
				response["model"] = publicModel
				if normalizedResponse, marshalErr := common.Marshal(response); marshalErr == nil {
					data["response"] = normalizedResponse
					changed = true
				}
			}
		}
	}
	if !changed {
		return taskDto
	}
	if normalizedData, err := common.Marshal(data); err == nil {
		taskDto.Data = normalizedData
	}
	return taskDto
}
