# 存储

Orla 将所有数据持久化在单个 Postgres 数据库中。Schema 是 daemon 与平台工程师 Mapper 之间的契约。

## 驱动与连接

- 使用 `github.com/jackc/pgx/v5` 及 `pgx/v5/stdlib` 适配器，使 `database/sql` 同时适用于应用查询与 `goose` 迁移。若后续 profiling 需要，热路径可使用原生 pgx。
- 连接 URL 通过 `ORLA_DATABASE_URL` 环境变量或 YAML 配置中的 `database_url` 指定。常规 Postgres URL：`postgres://user:pass@host:5432/orla?sslmode=...`。
- `*sql.DB` 默认配置已足够。`MaxOpenConns` 设为 `2 × backend.max_concurrency_sum`，即并发 dispatch goroutine 的上界，并为 API 与 BatchWriter 留出余量。`MaxIdleConns` 与之相等。`ConnMaxLifetime` 为 30 分钟，避免托管 Postgres 轮换连接时出问题。
- `sslmode=disable` 仅用于本地开发。生产环境按云厂商建议使用 `sslmode=require` 或 `sslmode=verify-full`。

## 迁移

使用 `github.com/pressly/goose/v3` 管理 `internal/storage/migrations/` 下嵌入的 `.sql` 文件。文件命名为 `NNNN_description.sql`，含 `-- +goose Up` 与 `-- +goose Down` 段。通过 `goose.SetDialect("postgres")` 设置方言。每次 `storage.Open` 时运行迁移，新数据库开箱即用。

## 写入策略

两类写入，耐久性需求不同：

| 类别 | 示例 | 写入模式 |
|---|---|---|
| 控制平面 | Stage 记录、backend 记录 | 同步 |
| 数据平面 | Completion 记录、feedback | 通过 `BatchWriter[T]` 异步批量 |

控制平面写入在行已持久化后才返回，调用方需要确认。

数据平面写入进入缓冲 channel，约每 100 行或每 100ms（以先到者为准）批量 flush。缓冲满时丢弃并计入 Prometheus 指标，**从不阻塞**生产者。Flush 使用 Postgres `COPY`（`pgx.CopyFrom`）以提升吞吐，这是相对逐行 `INSERT` 的主要收益。

> **注**：CompletionWriter 的默认配置为 buffer 4096、batch 200、interval 200ms，与 BatchWriter 包级默认值（1024/100/100ms）不同。详见 [`internal/telemetry/completion.go`](../internal/telemetry/completion.go)。

## Schema

### `stages`

平台工程师的操作面。

```sql
CREATE TABLE stages (
    id                TEXT PRIMARY KEY,
    backend           TEXT NOT NULL DEFAULT '',
    reasoning_effort  TEXT NOT NULL DEFAULT '',
    labels            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

首次见到 stage 时自动创建，字段为空。平台工程师通过 `PUT /api/v1/stages/{id}` 填写 `backend`，可选设置 `reasoning_effort` 与 `labels`。

`labels` 为自由 JSONB，Mapper 可在此存储自身状态（如上次动作时间戳、探索标志、arm 计数等），无需 schema 迁移。Mapper 可直接查询：

```sql
SELECT id FROM stages WHERE labels @> '{"exploring":true}'
```

### `backends`

```sql
CREATE TABLE backends (
    name                    TEXT PRIMARY KEY,
    endpoint                TEXT NOT NULL,
    model_id                TEXT,
    api_key_env_var         TEXT NOT NULL DEFAULT '',
    max_concurrency         INTEGER NOT NULL DEFAULT 1
                              CHECK (max_concurrency >= 1),
    rate_per_second         DOUBLE PRECISION,
    quality                 DOUBLE PRECISION,
    kind                    TEXT NOT NULL DEFAULT 'llm'
                              CHECK (kind IN ('llm', 'tool')),
    tool_kind               TEXT,
    input_cost_per_mtoken   DOUBLE PRECISION,
    output_cost_per_mtoken  DOUBLE PRECISION,
    rates                   JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`kind` 区分两类 backend：

- **`'llm'`**：OpenAI 兼容 chat completion。`model_id` 必填。成本来自 `input_cost_per_mtoken` 与 `output_cost_per_mtoken`。Orla proxy 在写入时计算 `cost_usd`：`(prompt_tokens × input_cost + completion_tokens × output_cost) / 1_000_000`。LLM backend 不使用 `rates` 列，注册时会被拒绝。
- **`'tool'`**：通过 HTTP 的 kind 特定 JSON RPC。`tool_kind` 标识工具族（如 `'structure-prediction'`、`'docking'` 等）。不使用 `model_id`。成本来自 `rates` JSONB（`resource_name` → 每单位 USD）。工具 wrapper 在响应中返回平行的 `usage` map，Orla 计算两 map 的点积作为 `cost_usd`。工具也可直接在响应中设置 `cost_usd`，经非负有限值校验后原样记录。

`quality` 是平台工程师提供的先验。Orla 不直接据此行动，Mapper 会读取它作为决策状态的一部分。

`max_concurrency` 是 Orla 直接强制执行的唯一运营上限。`rate_per_second` 按 Orla **进程**维度限流。

### `completion_records`

Mapper 的主要观测通道。

```sql
CREATE TABLE completion_records (
    completion_id     TEXT PRIMARY KEY,
    stage_id          TEXT NOT NULL,
    workflow_run      TEXT,
    backend           TEXT NOT NULL,
    status            TEXT NOT NULL,
    prompt_tokens     INTEGER,
    completion_tokens INTEGER,
    usage             JSONB NOT NULL DEFAULT '{}'::jsonb,
    tool_kind         TEXT,
    latency_ms        INTEGER,
    cost_usd          DOUBLE PRECISION,
    tags              JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_completion_stage_time ON completion_records(stage_id, created_at DESC);
CREATE INDEX idx_completion_workflow ON completion_records(workflow_run) WHERE workflow_run IS NOT NULL;
```

`status` 为 `"success"` 或 `"error"`。每次 `/v1/chat/completions` 或 `/v1/tools/{kind}` dispatch 写入一行，经 `BatchWriter` 异步写入。`tags` 原样保存 `X-Orla-Tag-*` 映射。

LLM 行填充 `prompt_tokens` 与 `completion_tokens`，`usage` 为空对象。Tool 行 token 列为 NULL，`usage` 填充 wrapper 报告的资源，`tool_kind` 设为 backend 的工具族。查询 tool 行时，用 `tool_kind IS NOT NULL` 而非 `prompt_tokens IS NULL` 过滤。两种情况下 `cost_usd` 均为 proxy 在写入时计算的最终美元金额。

默认不为 `tags` 添加 GIN 索引。多数 Mapper 查询先按 `stage_id` 过滤，b-tree 已覆盖。若 profiling 显示 tag 过滤查询是热点，可添加 `CREATE INDEX idx_completion_tags ON completion_records USING gin (tags)`。

### `feedback`

```sql
CREATE TABLE feedback (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    completion_id TEXT NOT NULL,
    stage_id      TEXT NOT NULL,
    workflow_run  TEXT,
    rating        DOUBLE PRECISION,
    labels        JSONB NOT NULL DEFAULT '[]'::jsonb,
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_feedback_completion ON feedback(completion_id);
CREATE INDEX idx_feedback_stage_time ON feedback(stage_id, created_at DESC);
```

`rating` 为 `[0, 1]` 或 `NULL`。由开发者提交，异步写入，端点返回 202。Mapper 通过 `completion_id` 与 `completion_records` 关联，将 feedback 归因到 backend。

## Mapper 如何使用这些数据

Schema 针对三种访问模式优化：

**按 stage 查询最近观测：**

```sql
SELECT * FROM completion_records
WHERE stage_id = $1 AND created_at > $2
ORDER BY created_at DESC LIMIT $3;
```

由 `idx_completion_stage_time` 支撑。

**Feedback 与 completion 关联：**

```sql
SELECT f.rating, c.backend, c.cost_usd, c.latency_ms
FROM feedback f
JOIN completion_records c USING (completion_id)
WHERE c.stage_id = $1 AND f.created_at > $2;
```

**按 (stage, backend) 聚合：**

```sql
SELECT backend,
       COUNT(*),
       AVG(latency_ms),
       SUM(cost_usd),
       PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY latency_ms) AS p50,
       PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95
FROM completion_records
WHERE stage_id = $1 AND created_at > $2
GROUP BY backend;
```

即 `/metrics` 端点暴露的内容。

## Mapper 的只读数据库访问

可 provision 第二个 Postgres 角色，对四张表仅有 `SELECT` 权限，供 REST 端点表达力不足、需要直连 Postgres 的 Mapper 使用：

```sql
CREATE ROLE orla_reader LOGIN PASSWORD '...';
GRANT CONNECT ON DATABASE orla TO orla_reader;
GRANT USAGE ON SCHEMA public TO orla_reader;
GRANT SELECT ON stages, backends, completion_records, feedback TO orla_reader;
```

REST API 仍是常见模式的权威入口。直连 SQL 是重量级分析查询的逃生舱，无需为每次查询新增端点。

## 部署

本地开发需要 Postgres 14+。通过系统包管理器安装后执行 `createdb orla`。本地默认 URL：`postgres://$(whoami)@localhost:5432/orla?sslmode=disable`。

生产环境使用任意 Postgres 14+ 部署，已在 RDS、Cloud SQL、Neon 等托管服务上测试。Orla 除默认 contrib 扩展外不需要额外扩展。

## 多 Orla 实例

Schema 支持 HA。多个 Orla 进程可指向同一数据库。

- 控制平面同步写入使用单行 upsert，并发写者由行锁自然串行化。
- BatchWriter 批次为仅追加 insert。completion id 冲突（UUID 下极罕见）通过 `ON CONFLICT DO NOTHING` 处理。
- Scheduler 队列在**每个实例进程内**。请求由负载均衡路由到某实例，从该实例的 worker pool dispatch。因此 per-backend `max_concurrency` 按**实例**而非全局强制执行。未来可通过 Postgres advisory lock 实现全局上限。

## 数据保留

无自动清理。Mapper 删除不需要的数据，或运行定期 vacuum 任务。Orla 不会主动删除观测数据，因为 Mapper 可能需要长期历史。

若 `completion_records` 增长成为问题，计划中的缓解方案是按 `created_at` 做 Postgres 原生分区。后续添加分区是非破坏性 schema 变更。

英文原文：[`storage.md`](storage.md)
