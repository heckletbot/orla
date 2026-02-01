# Orla overview

This page explains what Orla is and how its parts fit together. 

## What is Orla?

Orla is a layer that sits *above* LLM backends (Ollama, SGLang, vLLM) and provides **agent-granularity control** e.g which model and backend to use per step, how to manage inference state (e.g. KV cache), and how to coordinate multiple agents or workflow stages.

LLM serving systems like SGLang, vLLM, and Ollama optimize at the **request** level (prefill/decode, batching, per-request cache). Agentic systems decompose tasks into **multiple steps** that share context, have different computational requirements, and may span heterogeneous backends. Orla addresses that gap. It optimizes for *end-to-end* agent completion time and cost, with policies expressed per agent profile and per workflow stage.

## Three layers

Orla’s architecture envisions a three-layer stack:

### Hardware runtime layer
Executes the tensor programs (CUDA, Metal, ZenDNN, etc.). Platform- and model-dependent.

### LLM serving layer
Inference backends (Ollama, SGLang, vLLM) that run generation requests with decoding, batching, and caching.

### Agentic serving layer (Orla)

Sits above the LLM layer. Decides *what* to run, *where* to run it, and *how* to manage inference state across an agent or workflow’s steps. Does not reimplement transformer inference; it treats LLM servers as programmable execution engines.

The agentic serving layer is what you get when you run `orla daemon`. When you run `orla agent` without a daemon, a minimal version of this layer runs embedded so a single agent can still use one backend.

## Agent profiles

Orla’s core abstraction for agent-level control is **agent profiles**. An agent profile defines:

- Which **LLM server** (backend + model) to use
- **Inference options** (temperature, top_p, max_tokens)
- **Tool access** (optional allow-list)
- (When using the daemon) **Cache and context policies** at the LLM server level (e.g. preserve on small turns, flush under pressure, shared context for multi-agent)

Different workflow stages can use different profiles—e.g. a small model for routing and summarization, a larger model for synthesis—so you optimize per stage instead of one-size-fits-all. The daemon’s config ties workflows to agent profiles and LLM servers; see [Daemon and workflows](daemon.md).

## Modes of operation

Orla has three user-facing modes:

| Mode | Command | Role |
|------|---------|------|
| **Agent** | `orla agent` | Runs the agent loop: one agent, one backend (or embedded serving layer). Terminal, scripts, pipes. |
| **MCP server** | `orla serve` | Exposes Orla’s tools over the Model Context Protocol. No agent loop here—other MCP clients (Claude Desktop, Goose, etc.) use your tools. |
| **Daemon** | `orla daemon` | Runs the full Agentic Serving Layer: workflows, agent profiles, multi-agent coordination, shared context, KV cache policies. Clients drive workflows via HTTP API (StartWorkflow → GetNextTask → ExecuteTask → CompleteTask). |

You can combine them: e.g. run `orla serve` to give Claude Desktop access to Orla tools; run `orla daemon` when you need multi-agent workflows and agent-level policies.

## Where to go next

- [Daemon and workflows](daemon.md) — Configure and run `orla daemon`, define workflows and agent profiles
- [README](../README.md) — Installation, quickstart, and config for agent and serve
- [RFC 5: Agentic Serving Layer](rfcs/rfc5.txt) — Full configuration schema and operational model
- [pkg/api README](../pkg/api/README.md) — Go client for the daemon API
