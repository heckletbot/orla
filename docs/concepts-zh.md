# 概念

Orla 是一个 OpenAI 兼容的代理，位于 agent 代码与其调用的 LLM 或工具之间。它的职责是：把每次调用路由到当前正确的 backend、记录发生了什么，并允许外部 Mapper 在学习过程中动态调整这些路由。

本文说明概念模型。端到端上手见 [`quickstart-zh.md`](quickstart-zh.md)。

## 运行时自适应闭环

三个组件参与协作。Agent 发出带 stage 标签的 OpenAI 兼容调用，并在每次调用完成后提交评分。Orla 将调用转发到当前映射的 backend 并记录结果。Mapper 进程轮询 Orla、读取评分，当发现另一个 backend 更优时更新 stage 映射。

三者松耦合：Agent 不知道是哪台 backend 服务的；Mapper 不写 agent 代码。Orla 在中间维护共享记录，并通过 HTTP 同时服务两侧。

## Stage（阶段）

Stage 是开发者附加在每次调用上的标签，表示**这次调用在做什么**，而不是**发往哪里**。

一个五步研究型 agent 的 stage 划分示例：

| Stage | 调用用途 |
|---|---|
| `clarify` | 将用户问题改写为更精确的表述 |
| `plan` | 决定需要查找哪些证据 |
| `research` | 阅读语料并提取相关事实 |
| `compute` | 算术或多步推理 |
| `answer` | 综合生成最终回答 |

Stage 名称由开发者自行定义。Orla 在首次见到陌生 stage 名时自动创建记录，因此新增 stage 无需与平台侧协调。

为调用打标签：

```
X-Orla-Stage: research
```

对于不便设置 header 的 SDK，可在请求 body 中指定：

```json
{"metadata": {"orla": {"stage": "research"}}}
```

## Backend（后端）

Backend 是一个具体的推理端点，包含名称、OpenAI 兼容 URL、model id、API key 环境变量名、并发上限，以及平台工程师提供的成本与质量先验。

多模型部署中常见的 backend 示例：

| 名称 | 端点 | 说明 |
|---|---|---|
| `gpt-4o` | OpenAI | 前沿模型，高成本、高质量 |
| `gpt-4o-mini` | OpenAI | 中端，中等成本 |
| `qwen3-next-80b` | 自托管 | 开源权重模型，上下文窗口大 |
| `gemma-3-12b` | 自托管 | 开源权重模型，成本低 |

Backend 通过 API 注册。增删一个 backend 只需一次 HTTP 调用，无需重启 daemon。

## Mapping（映射）

映射回答的是：**stage X 当前应由哪个 backend 服务？** 它保存在 stage 记录中：

```json
{
  "id": "research",
  "backend": "qwen3-next-80b",
  "reasoning_effort": "",
  "labels": {}
}
```

用 `PUT /api/v1/stages/{id}` 设置，用 `PATCH /api/v1/stages/{id}` 修改。该 stage 的下一次调用立即使用新映射。

映射持久化在 Postgres 中。Orla 启动时会从数据库恢复，因此重启不会丢失 Mapper 已学习到的路由。

## Feedback（反馈）

调用完成后，agent 可以对结果评分：

```json
{
  "completion_id": "chatcmpl-abc",
  "stage_id": "research",
  "rating": 0.85
}
```

`rating` 取值范围为 `[0, 1]`，具体含义由你定义。常见做法包括：

- LLM-as-judge 对照标准答案打分
- 下游任务是否成功（如代码是否编译通过、工作流是否完成）
- 用户显式点赞/点踩
- 综合正确性与延迟的复合分数

同一 completion 允许多条 feedback，如何聚合由 Mapper 决定。

## Mapper（映射器）

Mapper 是你自己的代码，不是 Orla 内置的。它通过 Orla 的 HTTP API 读取 feedback 与 metrics，并决定 PATCH 什么。算法可选：

- **Epsilon-greedy bandit**：实现成本最低，适合 reward 曲面大致平稳的 stage。
- **Thompson sampling**：探索更平滑，适合 reward 信号噪声较大的场景。
- **Contextual bandit**：利用 prompt 长度、租户等请求特征。
- **延迟 reward 的 RL**：评分发生在调用很久之后时，off-policy RL 更合适。

Orla 不内置 Mapper，只提供 Mapper 所需的 API：

```
GET    /api/v1/stages/{id}/completions    # 原始 completion 记录
GET    /api/v1/stages/{id}/feedback       # 原始评分
GET    /api/v1/stages/{id}/metrics        # 按 backend 聚合
PATCH  /api/v1/stages/{id}                # 更新映射
```

一个简单的控制循环：

```python
while True:
    for stage_id in stages_to_manage:
        completions = orla.get_completions(stage_id)
        feedback   = orla.get_feedback(stage_id)
        chosen     = bandit.decide(stage_id, completions, feedback)
        if chosen != orla.get_stage(stage_id).backend:
            orla.patch_stage(stage_id, backend=chosen)
    time.sleep(30)
```

这就是完整的控制循环。Bandit 可以是二十行 Python，也可以是完整的 RL agent，Orla 一视同仁。

## 两种角色

Orla 围绕两种角色的严格分工构建。

**开发者**编写 agent 逻辑：为每次调用打上 stage 标签，可选附加 workflow 与 tenant 标签，**从不**选择 backend。

**平台工程师**注册 backend、设置初始映射、运行 Mapper 以随时间重映射 stage，**从不**修改 agent 代码。

各角色有独立的 API 面：开发者使用 `/v1/chat/completions` 代理和 `/v1/feedback`；平台工程师使用 `/api/v1/` 下的 REST CRUD 端点。

这种分离使运行时自适应变得安全：路由变更无需重新部署 agent；agent 变更无需平台工程师审核。Postgres 中的映射是两侧的共享边界。

## 身份标签

每次调用可通过 header 携带任意标签：

```
X-Orla-Stage: research
X-Orla-Workflow-Run: run-2026-06-04-abc123
X-Orla-Tag-Tenant: acme-corp
X-Orla-Tag-User: alice@acme
```

标签写入 `completion_records.tags_json`，Mapper 可读取。它们用于多租户公平调度、按客户仪表盘、按 workflow 分析等场景。

## Orla 不是什么

- **不是 agent 框架。** Orla 不解析 tool call、不跑循环、不做编排。它只提供 OpenAI 兼容请求服务。你的 agent 框架（LangGraph、LangChain、原生 OpenAI SDK 或自研框架）保持不变。
- **不是 model gateway。** Orla 不会根据 prompt 内容、成本阈值或 token 数自动选 backend。映射是平台工程师的杠杆，Orla 只负责执行。
- **不是 retry/fallback 链。** 解析出的 backend 在重试后仍失败，则调用失败。Fallback 策略属于 Mapper 职责。
- **不是 auth gateway。** 请在 nginx、Cloudflare 或服务网格之后运行 Orla 以处理认证。见 [`SECURITY.md`](../SECURITY.md)。

## 延伸阅读

- [`quickstart-zh.md`](quickstart-zh.md) — 动手搭建
- [`three-components.md`](three-components.md) — Stage Router、Telemetry、Runtime Mapper 的代码级解读
- [`proxy-zh.md`](proxy-zh.md) — `/v1/chat/completions` 的 wire 协议
- [`storage-zh.md`](storage-zh.md) — Mapper 读取的 Postgres schema

英文原文：[`concepts.md`](concepts.md)
