# 分组降智监测设计

用户已批准实现：左侧独立入口，管理员选择具体分组、目标模型、自定义题目和间隔分钟，开启/暂停/立即探测，保存答案和历史结果。先在本地验证品牌界面，再单独确认生产发布。

## 范围

- 仅允许 `gpt-6-astra`（用户简称 gpt-6-a）和 `gpt-6.1-sol`。
- 复用现有模型测试适配器与系统任务租约，按当前分组的有效模型能力、优先级/权重与协议兼容性选路，不写死渠道或分组名称。
- 管理员配置计划；普通登录用户只查看已发布且自己可使用分组的结果。管理员额外查看实际渠道、错误与估算费用。新计划默认暂停、结果默认不发布，避免保存即产生费用或公开自定义题目。
- 请求是无会话、非流式 `/v1/responses` 探测，题目来自管理员配置，思考强度/输出上限/超时显式冻结；可选择人工查看、不区分大小写包含或精确匹配参考答案。结果表示该次题目通过/未通过/调用失败，不宣称证明实际模型身份或整体能力。
- 目前使用支持输出预算的 OpenAI Compatible Responses 适配器。Codex 专用适配器会删除输出预算，因此显示为不支持并跳过；不强行改写其协议。
- 测试费用只写监测结果中的估算 quota，不扣任何用户/密钥余额，不加入普通消费日志或用户看板，不自动启停渠道、修改价格或其他项目。
- 全局任务串行避免重复探测/并发突刺；每轮按当前计划快照执行，计划修改不改历史题目/答案。过期租约后不自动重放已提交的收费请求。失败亦按完成时间安排下一次间隔，手动探测不隐式开启暂停计划。
- 单轮上限 100 次调用，最多 20 个分组、2 个模型、5 道题；题目和参考答案各 8 KiB/条，输出预算 128–8192，超时 10–180 秒。答案保存上限为 65,535 字节，兼容 MySQL TEXT，超限按 UTF-8 边界截断并明确标记，不自动判定通过。分页历史保留最近 30 天，单次最多删除 500 条，无启用计划时也可清理，清理只作用于监测结果。

## HTTP 契约

统一 `{success,message,data}`。读接口需要登录；写接口要求管理员。用户可见结果字段不含渠道、内部错误、费用与计划配置。

`GET /api/quality-monitor/options` → `{models:string[],groups:{name:string,models:string[]}[]}`，管理员使用有效渠道能力列表，普通用户仅可用分组。

`GET /api/quality-monitor/plans`（管理员） → `QualityMonitorPlan[]`。

`POST /api/quality-monitor/plans`、`PUT /api/quality-monitor/plans/:id`（管理员）接收下述配置，后端校验全部字段、模型白名单、组合次数和分组有效性。没有覆盖的分组/模型组合显示跳过而非错误路由。

`POST /api/quality-monitor/plans/:id/run`（管理员） → `{task_id:string}`；正在运行时返回冲突，不重复发起。`DELETE /api/quality-monitor/plans/:id`（管理员）删除计划，保留历史结果。

`GET /api/quality-monitor/results?group=&model=&plan_id=&page=1&page_size=20` → `{items:QualityMonitorResult[],total:number}`；权限筛选在数据库查询层，不能靠前端隐藏。

```ts
type QualityMonitorQuestion = {
  id: string; prompt: string; expected_answer: string;
  match_type: 'manual' | 'exact' | 'contains';
}
type QualityMonitorPlan = {
  id: number; name: string; enabled: boolean; published: boolean;
  groups: string[]; models: string[]; interval_minutes: number;
  reasoning_effort: 'low' | 'medium' | 'high' | 'max';
  max_output_tokens: number; timeout_seconds: number;
  questions: QualityMonitorQuestion[];
  last_run_at: number; next_run_at: number; created_at: number; updated_at: number;
}
type QualityMonitorResult = {
  id: number; plan_id: number; plan_name: string; group: string; model: string;
  question_id: string; prompt: string; expected_answer: string; answer: string;
  status: 'passed' | 'failed' | 'ungraded' | 'error' | 'skipped';
  duration_ms: number; created_at: number; answer_truncated: boolean;
  reasoning_effort: string; channel_id?: number; error?: string;
  actual_response_model?: string; estimated_quota?: number;
}
```

## 页面

`/quality-monitor` 在现有 console 左侧入口新增“降智监测”。所有人有分组/模型筛选、最近题目结果、历史列表和可展开/复制答案；管理员额外显示“监测计划”页签，支持创建/编辑、多选分组与模型、间隔预设和自定义、题目和判定规则、暂停/开启、立即探测。沿用生产品牌容器、主题变量、字体、背景与微动效，手机端内容自然滚动，浅/深色对比可读。

不增加重依赖或额外 Docker 服务。新增文案经六语言 i18n 脚本写入，结果分页且不自动循环真实请求。预览只连接隔离本地数据库与可控模拟上游。
