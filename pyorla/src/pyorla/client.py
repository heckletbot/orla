"""HTTP client for the Orla server. Mirrors Go OrlaClient (pkg/api/client.go)."""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator
from typing import Any

import httpx

from pyorla.types import (
    ExecuteRequest,
    InferenceResponse,
    InferenceResponseMetrics,
    LLMBackend,
    StreamEvent,
)


class OrlaError(Exception):
    """Raised when the Orla server returns an error."""


class OrlaClient:
    """Sync + async HTTP client for the Orla daemon.

    Mirrors Go ``OrlaClient`` from ``pkg/api/client.go``.
    """

    def __init__(self, base_url: str = "http://localhost:8081") -> None:
        self.base_url = base_url.rstrip("/")
        self._sync = httpx.Client(base_url=self.base_url, timeout=300)
        self._async = httpx.AsyncClient(base_url=self.base_url, timeout=300)

    # ------------------------------------------------------------------
    # Health
    # ------------------------------------------------------------------

    def health(self) -> None:
        """Check Orla daemon health. Raises on failure."""
        resp = self._sync.get("/api/v1/health")
        resp.raise_for_status()

    async def ahealth(self) -> None:
        resp = await self._async.get("/api/v1/health")
        resp.raise_for_status()

    # ------------------------------------------------------------------
    # Register backend
    # ------------------------------------------------------------------

    def register_backend(self, backend: LLMBackend) -> None:
        """Register an LLM backend with the daemon."""
        payload = _backend_to_dict(backend)
        resp = self._sync.post("/api/v1/backends", json=payload)
        resp.raise_for_status()
        data = resp.json()
        if not data.get("success"):
            raise OrlaError(f"register backend failed: {data.get('error', 'unknown')}")

    async def aregister_backend(self, backend: LLMBackend) -> None:
        payload = _backend_to_dict(backend)
        resp = await self._async.post("/api/v1/backends", json=payload)
        resp.raise_for_status()
        data = resp.json()
        if not data.get("success"):
            raise OrlaError(f"register backend failed: {data.get('error', 'unknown')}")

    # ------------------------------------------------------------------
    # Execute (non-streaming)
    # ------------------------------------------------------------------

    def execute(self, req: ExecuteRequest) -> InferenceResponse:
        """Run inference on a named backend. Mirrors Go OrlaClient.Execute."""
        payload = req.to_dict()
        resp = self._sync.post("/api/v1/execute", json=payload)
        resp.raise_for_status()
        return _parse_execute_response(resp.json())

    async def aexecute(self, req: ExecuteRequest) -> InferenceResponse:
        payload = req.to_dict()
        resp = await self._async.post("/api/v1/execute", json=payload)
        resp.raise_for_status()
        return _parse_execute_response(resp.json())

    # ------------------------------------------------------------------
    # Execute (streaming)
    # ------------------------------------------------------------------

    def execute_stream(self, req: ExecuteRequest) -> Iterator[StreamEvent]:
        """Run streaming inference. Mirrors Go OrlaClient.ExecuteStream."""
        payload = req.to_dict()
        payload["stream"] = True
        with self._sync.stream("POST", "/api/v1/execute", json=payload) as resp:
            resp.raise_for_status()
            yield from _iter_sse_events(resp.iter_lines())

    async def aexecute_stream(self, req: ExecuteRequest) -> AsyncIterator[StreamEvent]:
        payload = req.to_dict()
        payload["stream"] = True
        async with self._async.stream("POST", "/api/v1/execute", json=payload) as resp:
            resp.raise_for_status()
            async for line in resp.aiter_lines():
                ev = _parse_sse_line(line, {})
                if ev is not None:
                    yield ev

    # ------------------------------------------------------------------
    # Workflow complete
    # ------------------------------------------------------------------

    def workflow_complete(self, workflow_id: str, backends: list[str]) -> None:
        """Notify the server a workflow has finished."""
        resp = self._sync.post(
            "/api/v1/workflow/complete",
            json={"workflow_id": workflow_id, "backends": backends},
        )
        resp.raise_for_status()

    async def aworkflow_complete(self, workflow_id: str, backends: list[str]) -> None:
        resp = await self._async.post(
            "/api/v1/workflow/complete",
            json={"workflow_id": workflow_id, "backends": backends},
        )
        resp.raise_for_status()

    # ------------------------------------------------------------------
    # Cleanup
    # ------------------------------------------------------------------

    def close(self) -> None:
        self._sync.close()

    async def aclose(self) -> None:
        await self._async.aclose()


# ======================================================================
# Helpers
# ======================================================================

def _backend_to_dict(b: LLMBackend) -> dict[str, Any]:
    d: dict[str, Any] = {
        "name": b.name,
        "endpoint": b.endpoint,
        "type": b.type,
        "model_id": b.model_id,
    }
    if b.api_key_env_var:
        d["api_key_env_var"] = b.api_key_env_var
    if b.max_concurrency > 1:
        d["max_concurrency"] = b.max_concurrency
    if b.queue_capacity > 0:
        d["queue_capacity"] = b.queue_capacity
    return d


def _parse_execute_response(data: dict) -> InferenceResponse:
    if not data.get("success"):
        raise OrlaError(f"execution failed: {data.get('error', 'unknown')}")
    r = data.get("response", {})
    metrics = None
    if "metrics" in r:
        m = r["metrics"]
        metrics = InferenceResponseMetrics(
            ttft_ms=m.get("ttft_ms", 0),
            tpot_ms=m.get("tpot_ms", 0),
            prompt_tokens=m.get("prompt_tokens", 0),
            completion_tokens=m.get("completion_tokens", 0),
            queue_wait_ms=m.get("queue_wait_ms", 0),
            scheduler_decision_ms=m.get("scheduler_decision_ms", 0),
            dispatch_ms=m.get("dispatch_ms", 0),
            backend_latency_ms=m.get("backend_latency_ms", 0),
        )
    tool_calls = r.get("tool_calls") or []
    return InferenceResponse(
        content=r.get("content", ""),
        thinking=r.get("thinking", ""),
        tool_calls=tool_calls,
        metrics=metrics,
    )


import json as _json  # noqa: E402


def _iter_sse_events(lines: Iterator[str]) -> Iterator[StreamEvent]:
    """Parse SSE text/event-stream lines into StreamEvents."""
    state: dict[str, str] = {}
    for line in lines:
        ev = _parse_sse_line(line, state)
        if ev is not None:
            yield ev


def _parse_sse_line(line: str, state: dict[str, str]) -> StreamEvent | None:
    if line.startswith("event: "):
        state["event"] = line[7:]
        return None
    if line.startswith("data: "):
        state["data"] = line[6:]
        return None
    if line == "" and "event" in state and "data" in state:
        ev = _build_stream_event(state["event"], state["data"])
        state.clear()
        return ev
    return None


def _build_stream_event(event_type: str, data_str: str) -> StreamEvent | None:
    try:
        data = _json.loads(data_str)
    except _json.JSONDecodeError:
        return None

    if event_type == "content":
        return StreamEvent(type="content", content=data.get("content", ""))
    if event_type == "thinking":
        return StreamEvent(type="thinking", thinking=data.get("thinking", ""))
    if event_type == "tool_call":
        return StreamEvent(
            type="tool_call",
            tool_call={"name": data.get("name", ""), "arguments": data.get("arguments", {})},
        )
    if event_type == "done":
        if data.get("success") and data.get("response"):
            return StreamEvent(
                type="done",
                response=_parse_execute_response(data),
            )
    return None
