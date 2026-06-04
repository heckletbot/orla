-- +goose Up

-- Stage records. The platform engineer's action surface.
CREATE TABLE stages (
    id                TEXT PRIMARY KEY,
    backend           TEXT NOT NULL DEFAULT '',
    reasoning_effort  TEXT NOT NULL DEFAULT '',
    labels            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Backend records. One row per concrete inference endpoint.
--
-- kind:
--   'llm'  OpenAI-compatible chat completion. Uses model_id and the
--          per-million-token cost columns.
--   'tool' Scientific computation tool such as structure prediction.
--          Uses tool_kind to identify the tool family and `rates` for
--          arbitrary per-resource pricing.
--
-- rates is a JSONB map of resource_name to USD-per-unit. Tools report
-- usage as a parallel map, and orla computes cost as the dot product.
-- Examples:
--   {"gpu_seconds": 0.001}        a GPU-billed tool
--   {"cpu_seconds": 0.0001}       a CPU-billed tool
--   {"calls": 0.005}              a flat per-call billed API
--   {"gpu_seconds": 0.001, "calls": 0.0001}   mixed
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

-- One row per dispatched request. LLM rows populate prompt_tokens and
-- completion_tokens. Tool rows leave those NULL and populate `usage`
-- with the resource counts the tool wrapper reported. cost_usd is
-- always the final dollar amount, computed at write time.
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
CREATE INDEX idx_completion_stage_time
    ON completion_records(stage_id, created_at DESC);
CREATE INDEX idx_completion_workflow
    ON completion_records(workflow_run) WHERE workflow_run IS NOT NULL;

-- Developer-submitted ratings, joinable to completion_records by
-- completion_id.
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

-- +goose Down
DROP TABLE feedback;
DROP TABLE completion_records;
DROP TABLE backends;
DROP TABLE stages;
