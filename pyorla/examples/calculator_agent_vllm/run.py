"""Calculator agent using LangGraph, pyorla, and vLLM in Docker.

This example is self-contained: it builds the local ``Dockerfile``, runs vLLM with the Docker
Python SDK, waits for the server to become healthy, starts Orla via ``orla_runtime()``, then
runs the calculator graph.

Prerequisites:

- Docker Engine running (the ``docker`` CLI is not required; the SDK talks to the daemon).
- On Linux with NVIDIA GPUs, the NVIDIA Container Toolkit so ``device_requests`` GPU passthrough works.
- The ``orla`` binary on ``PATH`` (or ``ORLA_BIN``).

The image serves ``Qwen/Qwen3-4B-Instruct-2507`` on port 8000, matching the ``Dockerfile`` ``CMD``.

Run from the ``pyorla`` directory::

    uv run python examples/calculator_agent_vllm/run.py
"""

from __future__ import annotations

import contextlib
import logging
import os
import time
from collections.abc import Iterator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Annotated, Any, Literal

import docker
import httpx
from docker.errors import APIError, DockerException
from docker.types import DeviceRequest
from langchain_core.messages import (
    AIMessage,
    AnyMessage,
    HumanMessage,
    SystemMessage,
    ToolMessage,
)
from langchain_core.tools import tool
from langgraph.graph import END, START, StateGraph
from langgraph.graph.message import add_messages

from pyorla import (
    OrlaBinaryNotFoundError,
    OrlaClient,
    OrlaError,
    Stage,
    new_vllm_backend,
    orla_runtime,
)

log = logging.getLogger(__name__)

DOCKER_IMAGE = "orla-calc-vllm:dev"
_VLLM_HEALTH = "http://127.0.0.1:8000/health"


def _drain_image_build(log_iter: Iterator[dict[str, Any]]) -> None:
    """Consume Docker ``images.build`` JSON stream and log progress; raise on error chunks."""
    for chunk in log_iter:
        if err := chunk.get("errorDetail"):
            raise RuntimeError(err.get("message", str(chunk)))
        if stream := chunk.get("stream"):
            line = stream.rstrip()
            if line:
                log.info("%s", line)
        if status := chunk.get("status"):
            extra = chunk.get("progress") or chunk.get("id")
            if extra:
                log.info("%s %s", status, extra)
            else:
                log.info("%s", status)


@contextlib.contextmanager
def vllm_docker_runtime() -> Iterator[None]:
    """Build ``Dockerfile``, run vLLM with GPU, wait for ``/health``, stop on exit."""
    root = Path(__file__).resolve().parent
    if not (root / "Dockerfile").is_file():
        raise RuntimeError(f"No Dockerfile in {root}")

    log.info("Building Docker image %s from %s ...", DOCKER_IMAGE, root)
    client = docker.from_env()
    _, log_iter = client.images.build(
        path=str(root), dockerfile="Dockerfile", tag=DOCKER_IMAGE, rm=True
    )
    _drain_image_build(log_iter)
    log.info("Docker image %s ready.", DOCKER_IMAGE)

    cname = f"orla-calc-vllm-{os.getpid()}"
    log.info("Starting container %s (GPU, port 8000 -> host 8000) ...", cname)
    container = client.containers.run(
        DOCKER_IMAGE,
        detach=True,
        remove=True,
        name=cname,
        ports={"8000/tcp": 8000},
        device_requests=[DeviceRequest(count=-1, capabilities=[["gpu"]])],
    )
    log.info("Container %s is up (id %s).", cname, container.short_id)
    try:
        log.info("Waiting for vLLM health at %s ...", _VLLM_HEALTH)
        for attempt in range(1, 201):
            try:
                if httpx.get(_VLLM_HEALTH, timeout=5).status_code == 200:
                    log.info("vLLM reported healthy after %d check(s).", attempt)
                    break
            except httpx.HTTPError:
                pass
            if attempt % 10 == 0:
                log.info("Still waiting for vLLM (check %d / 200) ...", attempt)
            time.sleep(3)
        else:
            raise RuntimeError(f"vLLM did not become healthy at {_VLLM_HEALTH}")
        yield
    finally:
        log.info("Stopping container %s ...", cname)
        try:
            container.stop(timeout=60)
        except APIError:
            pass
        log.info("Container stopped.")


@tool
def multiply(a: int, b: int) -> int:
    """Multiply `a` and `b`."""
    return a * b


@tool
def add(a: int, b: int) -> int:
    """Adds `a` and `b`."""
    return a + b


@tool
def divide(a: int, b: int) -> float:
    """Divide `a` and `b`."""
    return a / b


@dataclass
class CalculatorState:
    """Shared graph state: messages (with LangGraph's ``add_messages`` reducer) plus call count."""

    messages: Annotated[list[AnyMessage], add_messages] = field(default_factory=list)
    llm_calls: int = 0


def build_graph(model_with_tools, tools_by_name: dict):
    def llm_call(state: CalculatorState):
        return {
            "messages": [
                model_with_tools.invoke(
                    [
                        SystemMessage(
                            content=(
                                "You are a helpful assistant tasked with performing "
                                "arithmetic on a set of inputs."
                            )
                        )
                    ]
                    + state.messages
                )
            ],
            "llm_calls": state.llm_calls + 1,
        }

    def tool_node(state: CalculatorState):
        last = state.messages[-1]
        if not isinstance(last, AIMessage) or not last.tool_calls:
            return {"messages": []}
        result = []
        for tool_call in last.tool_calls:
            t = tools_by_name[tool_call["name"]]
            observation = t.invoke(tool_call["args"])
            result.append(
                ToolMessage(content=str(observation), tool_call_id=tool_call["id"])
            )
        return {"messages": result}

    def should_continue(state: CalculatorState) -> Literal["tool_node", "__end__"]:
        last_message = state.messages[-1]
        if isinstance(last_message, AIMessage) and last_message.tool_calls:
            return "tool_node"
        return "__end__"

    agent_builder = StateGraph(CalculatorState)
    agent_builder.add_node("llm_call", llm_call)
    agent_builder.add_node("tool_node", tool_node)
    agent_builder.add_edge(START, "llm_call")
    agent_builder.add_conditional_edges(
        "llm_call",
        should_continue,
        ["tool_node", END],
    )
    agent_builder.add_edge("tool_node", "llm_call")
    return agent_builder.compile()


def _run_calculator(client: OrlaClient) -> None:
    tools = [add, multiply, divide]
    tools_by_name = {t.name: t for t in tools}

    vllm = new_vllm_backend("Qwen/Qwen3-4B-Instruct-2507", "http://127.0.0.1:8000/v1")
    client.register_backend(vllm)

    stage = Stage("calculator", vllm)
    stage.client = client
    stage.set_max_tokens(512)
    stage.set_temperature(0)
    model_with_tools = stage.as_chat_model().bind_tools(tools)

    agent = build_graph(model_with_tools, tools_by_name)
    out = agent.invoke({"messages": [HumanMessage(content="Add 3 and 4.")]})
    for m in out["messages"]:
        m.pretty_print()


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(message)s")
    try:
        with vllm_docker_runtime():
            with orla_runtime(quiet=True) as client:
                _run_calculator(client)
    except OrlaBinaryNotFoundError as exc:
        raise SystemExit(
            f"{exc}\n"
            "Install Orla, put `orla` on PATH, or set ORLA_BIN to the binary path."
        ) from exc
    except OrlaError as exc:
        raise SystemExit(str(exc)) from exc
    except RuntimeError as exc:
        raise SystemExit(str(exc)) from exc
    except DockerException as exc:
        raise SystemExit(
            f"{exc}\n"
            "Start Docker, and on Linux use NVIDIA Container Toolkit for GPU containers."
        ) from exc


if __name__ == "__main__":
    main()
