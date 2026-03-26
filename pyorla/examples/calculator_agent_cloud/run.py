"""Calculator agent using LangGraph, pyorla, and Amazon Bedrock Mantle (OpenAI-compatible API).

Prerequisites:

- The ``orla`` CLI on ``PATH`` (or ``ORLA_BIN``); this script starts a local daemon with
  ``orla_runtime()`` (subprocess ``orla serve`` on a free port).
- ``OPENAI_API_KEY`` — API key for the OpenAI-compatible Bedrock Mantle endpoint (Bearer token).
  Export it in the same shell before running so the spawned ``orla`` process inherits it.
- A Bedrock foundation model enabled for your account that supports Chat Completions and tool use.

Run from the ``pyorla`` directory::

    uv run python examples/calculator_agent_cloud/run.py
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import Annotated, Literal

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
    LLMBackend,
    OrlaBinaryNotFoundError,
    OrlaClient,
    OrlaError,
    Stage,
    orla_runtime,
)

MANTLE_BASE_URL = "https://bedrock-mantle.us-east-2.api.aws/v1"
# Orla provider prefix + Mantle chat model id
MANTLE_MODEL_ID = "openai:mistral.ministral-3-3b-instruct"


def _env(key: str, default: str) -> str:
    return os.environ.get(key, default).strip()


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

    bedrock = LLMBackend(
        name="bedrock-mantle",
        endpoint=MANTLE_BASE_URL,
        type="openai",
        model_id=MANTLE_MODEL_ID,
        api_key_env_var="OPENAI_API_KEY",
    )
    client.register_backend(bedrock)

    stage = Stage("calculator", bedrock)
    stage.client = client
    stage.set_max_tokens(512)
    stage.set_temperature(0)
    model_with_tools = stage.as_chat_model().bind_tools(tools)

    agent = build_graph(model_with_tools, tools_by_name)
    out = agent.invoke({"messages": [HumanMessage(content="Add 3 and 4.")]})
    for m in out["messages"]:
        m.pretty_print()


def main() -> None:
    if not _env("OPENAI_API_KEY", ""):
        raise SystemExit(
            "OPENAI_API_KEY must be set in the environment before `orla serve` starts "
            "(the daemon reads it when calling the Bedrock Mantle OpenAI-compatible API)."
        )

    try:
        with orla_runtime(quiet=True) as client:
            _run_calculator(client)
    except OrlaBinaryNotFoundError as exc:
        raise SystemExit(
            f"{exc}\n"
            "Install Orla, put `orla` on PATH, or set ORLA_BIN to the binary path."
        ) from exc
    except OrlaError as exc:
        raise SystemExit(str(exc)) from exc


if __name__ == "__main__":
    main()
