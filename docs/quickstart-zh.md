# 快速上手

从零开始，大约十分钟内跑通一个具备运行时自适应能力的 agent。

## 前置条件

- Go 1.26+
- 运行中的 Postgres 14+ 实例
- 至少一个 OpenAI 兼容模型提供商的 API key

## 1. 安装 daemon

```bash
go install github.com/harvard-cns/orla/cmd/orla@latest
```

或从源码构建：

```bash
git clone https://github.com/harvard-cns/orla
cd orla
go build -o bin/orla ./cmd/orla
go build -o bin/orlactl ./cmd/orlactl
```

## 2. 启动 daemon

将 Orla 指向你的 Postgres：

```bash
export ORLA_DATABASE_URL="postgres://user:pass@localhost:5432/orla?sslmode=disable"
orla serve
```

stderr 上应出现结构化日志。HTTP API 默认监听 `localhost:8081`，可通过 `ORLA_LISTEN_ADDRESS` 覆盖。

健康检查：

```bash
curl http://localhost:8081/healthz   # 存活探针
curl http://localhost:8081/readyz    # 就绪探针，同时 ping Postgres
```

## 3. 注册 backend

Backend 是一个推理端点。告诉 Orla 它的地址、读取哪个 API key 环境变量、以及并发上限：

```bash
curl -X POST http://localhost:8081/api/v1/backends \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "gpt-4o",
    "endpoint": "https://api.openai.com/v1",
    "model_id": "openai:gpt-4o",
    "api_key_env_var": "OPENAI_API_KEY",
    "max_concurrency": 8,
    "quality": 0.9,
    "rate_per_second": 10
  }'
```

`api_key_env_var` 是 dispatch 时 Orla 读取的环境变量名。Orla **不存储** API key 本身。

用同样方式注册第二个 backend。有两个以上选项时，运行时自适应的故事才更有意思。

## 4. 将 stage 映射到 backend

Stage 是 agent 附加在每次 LLM 调用上的标签。在流量到达前，将 stage 映射到 backend：

```bash
curl -X PUT http://localhost:8081/api/v1/stages/planning \
  -H 'Content-Type: application/json' \
  -d '{"backend": "gpt-4o"}'
```

若跳过此步，首个带 `X-Orla-Stage: planning` 的请求会自动创建无 backend 的 stage 记录，该次调用回退到请求中的 `model` 字段。

## 5. 通过 Orla 发送 chat completion

将任意 OpenAI 兼容客户端指向 Orla，并添加一个 header：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8081/v1",
    api_key="anything",
)

resp = client.chat.completions.create(
    model="ignored",
    messages=[{"role": "user", "content": "Summarize the second amendment."}],
    extra_headers={"X-Orla-Stage": "planning"},
)
print(resp.choices[0].message.content)
print("resolved backend:", resp.model)
```

`resp.model` 报告实际处理请求的 backend。请求中的 `model` 字段仅作为无映射 stage 的回退值。

## 6. 提交 feedback

Agent 对输出打分后，告诉 Orla 这次调用表现如何。评分可以是 LLM judge、下游任务成功信号、用户点赞，或任何你定义的方式。

```bash
curl -X POST http://localhost:8081/v1/feedback \
  -H 'Content-Type: application/json' \
  -d '{
    "completion_id": "chatcmpl-abc",
    "stage_id": "planning",
    "rating": 0.8
  }'
```

`rating` 为 `[0, 1]` 区间的数值，越高越好。超出范围会被拒绝。

你接入的 bandit 或 Mapper 读取 feedback 并重映射 stage。

## 7. 观察自适应过程

查看 Orla 在该 stage 上积累的数据：

```bash
# 最近的 completion，按时间倒序
curl 'http://localhost:8081/api/v1/stages/planning/completions?limit=50'

# feedback，可通过 completion_id 与 completion 关联
curl 'http://localhost:8081/api/v1/stages/planning/feedback?limit=50'

# 按 backend 聚合，可直接用于 reward 函数
curl 'http://localhost:8081/api/v1/stages/planning/metrics'
```

当 Mapper 判定另一个 backend 更优时，PATCH stage：

```bash
curl -X PATCH http://localhost:8081/api/v1/stages/planning \
  -H 'Content-Type: application/json' \
  -d '{"backend": "gpt-4o-mini"}'
```

下一次 `planning` 请求即路由到新 backend。无需重启，无需修改 agent 代码。

## 延伸阅读

- [`concepts-zh.md`](concepts-zh.md) — stage、backend 与 feedback 闭环的概念模型
- [`three-components.md`](three-components.md) — Stage Router、Telemetry、Runtime Mapper 代码解读
- [`proxy-zh.md`](proxy-zh.md) — `/v1/chat/completions` 完整 wire 协议
- [`storage-zh.md`](storage-zh.md) — Mapper 读取的 Postgres schema

英文原文：[`quickstart.md`](quickstart.md)
