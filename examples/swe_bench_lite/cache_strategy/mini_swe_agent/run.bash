#!/usr/bin/env bash
set -e

# Use SGLang's OpenAI-compatible API (same as Orla). LiteLLM uses OPENAI_BASE_URL and openai/<model>.
OPENAI_BASE_URL="${OPENAI_BASE_URL:-http://localhost:30000/v1}"
# Dummy key for local SGLang (no auth required)
OPENAI_API_KEY="${OPENAI_API_KEY:-sk-no-key-required}"

# Model: openai/ prefix routes to OpenAI-compatible endpoint; name must match what SGLang serves.
# Same model as Orla (Qwen/Qwen3-8B) for comparable runs; use same docker-compose.sglang.yaml.
MODEL="${MINI_SWE_MODEL:-openai/Qwen/Qwen3-8B}"
OUTPUT_DIR="mini_swe_agent_preds"

# Local/custom models (e.g. Qwen3-8B) aren't in LiteLLM's cost map; avoid RuntimeError.
MSWEA_COST_TRACKING="${MSWEA_COST_TRACKING:-ignore_errors}"

export OPENAI_BASE_URL OPENAI_API_KEY MSWEA_COST_TRACKING

exec mini-extra swebench \
  --model "$MODEL" \
  --subset lite \
  --split dev \
  -o "$OUTPUT_DIR" \
  "$@"
