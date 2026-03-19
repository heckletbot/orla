"""Tests for pyorla.tools."""

from pyorla.tools import (
    Tool,
    ToolCall,
    ToolResult,
    new_tool,
    tool_call_from_raw,
    tool_runner_from_schema,
)


def test_tool_to_mcp():
    t = Tool(name="greet", description="Says hi", input_schema={"type": "object"})
    mcp = t.to_mcp()
    assert mcp["name"] == "greet"
    assert mcp["description"] == "Says hi"
    assert "inputSchema" in mcp
    assert "outputSchema" not in mcp


def test_tool_to_mcp_with_output_schema():
    t = Tool(
        name="greet",
        description="Says hi",
        input_schema={"type": "object"},
        output_schema={"type": "string"},
    )
    mcp = t.to_mcp()
    assert mcp["outputSchema"] == {"type": "string"}


def test_tool_call_from_raw_mcp_envelope():
    raw = {
        "id": "tc-1",
        "method": "tools/call",
        "params": {"name": "read_file", "arguments": {"path": "/tmp/x"}},
    }
    tc = tool_call_from_raw(raw)
    assert tc.id == "tc-1"
    assert tc.name == "read_file"
    assert tc.input_arguments == {"path": "/tmp/x"}


def test_tool_call_from_raw_flat():
    raw = {"id": "tc-2", "name": "search", "arguments": {"q": "hello"}}
    tc = tool_call_from_raw(raw)
    assert tc.name == "search"
    assert tc.input_arguments == {"q": "hello"}


def test_tool_runner_from_schema_success():
    def fn(inp):
        return {"result": inp.get("x", 0) * 2}

    runner = tool_runner_from_schema(fn)
    result = runner({"x": 5})
    assert result.output_values == {"result": 10}
    assert not result.is_error


def test_tool_runner_from_schema_error():
    def fn(_inp):
        raise ValueError("boom")

    runner = tool_runner_from_schema(fn)
    result = runner({})
    assert result.is_error
    assert "boom" in result.error


def test_tool_result_to_message_dict():
    r = ToolResult(id="tc-1", name="greet", output_values={"msg": "hi"})
    msg = r.to_message_dict()
    assert msg["role"] == "tool"
    assert msg["tool_call_id"] == "tc-1"
    assert msg["tool_name"] == "greet"
    assert '"msg"' in msg["content"]


def test_tool_result_error_to_message_dict():
    r = ToolResult(id="tc-2", name="fail", error="bad input", is_error=True)
    msg = r.to_message_dict()
    assert "tool execution error" in msg["content"]
    assert "bad input" in msg["content"]
