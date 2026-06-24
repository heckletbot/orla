# 代理：`POST /v1/chat/completions`

OpenAI 兼容的推理入口。将任意 OpenAI 兼容客户端指向 Orla，并添加 stage header 即可。

## Wire 格式

**请求**：标准 OpenAI chat completion，附加身份元数据。完整标签列表见 [`concepts-zh.md`](concepts-zh.md#身份标签)。

**响应**：标准 OpenAI chat completion。响应中的 `model` 字段报告的是**实际解析出的 backend 名称**，不一定与客户端请求 body 中的 `model` 一致。在 [`concepts-zh.md`](concepts-zh.md) 所述的开发者/平台工程师分工下，这种差异是刻意设计的。

流式响应遵循 OpenAI 的纯 data SSE 格式，以 `data: [DONE]` 结束。

## 请求处理顺序

Handler 按以下顺序执行检查（未特别说明时，失败均返回 400）：

1. **解析 body。** 超过 10 MB 返回 400。
2. **`messages` 非空。**
3. **提取 stage。** 优先从 `X-Orla-Stage` header 读取，否则从 body 的 `metadata.orla.stage` 读取。缺失则返回 400。
4. **解析 backend。** `registry.GetOrCreate(stage)` 在首次见到 stage 时自动创建默认记录。若 `stage.Backend` 已设置则使用它；否则回退到 `req.Model`；两者皆空则返回 400。
5. **应用推理策略**，来自 stage 记录，目前仅 `reasoning_effort`。
6. **转换 messages 与 tools** 为内部模型类型。
7. **Dispatch**：经 `LayerExecute`、`BackendManager.ScheduleChat`，进入 per-backend 队列，由 worker 调用 openai-go provider。
8. **编码响应** 为 OpenAI chat completion 或 stream chunk。

## 首次见到 stage 时自动创建

若开发者使用的 stage id 是 Orla 从未见过的，daemon 会插入一条 backend 为空的默认 stage 记录，该次请求回退到 `req.Model`。平台工程师之后可通过 `PUT /api/v1/stages/{id}` 为其建立映射。

这意味着开发者可以部署新 agent 代码而无需与平台工程师协调：请求照常流转，Mapper 在下一轮扫描时会接管该 stage。

## 身份标签写入 completion 记录

每次 dispatch 在 `completion_records` 中写入一行，字段包括：

- `completion_id`：Orla 分配的 UUID
- `stage_id`：来自请求
- `workflow_run`：来自请求，可为 NULL
- `backend`：解析出的 backend 名称
- `tags_json`：完整的 `X-Orla-Tag-*` 映射
- `prompt_tokens`、`completion_tokens`、`latency_ms`、`cost_usd`、`status`、`created_at`

这是 Mapper 的主要观测通道。详见 [`storage-zh.md`](storage-zh.md)。

## 流式语义

当 `stream: true` 时：

- Orla 打开上游 stream 并代理 chunk；每个 chunk 的 `model` 字段会被改写为解析出的 backend 名称。
- Worker 的并发 slot 会持有到**上游 stream 完全 drain**，而不仅是收到第一个 chunk。这对 `max_concurrency` 不变量至关重要。
- 客户端断开时，Orla 静默 drain 上游 stream，随后释放 worker slot。

## 错误格式

所有非 200 响应使用 OpenAI 错误 envelope：

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error" | "permission_denied" | "rate_limit_exceeded" | "server_error" | "api_error"
  }
}
```

HTTP 状态码决定 `type` 字段。已能处理 OpenAI 错误的客户端无需额外适配。

英文原文：[`proxy.md`](proxy.md)
