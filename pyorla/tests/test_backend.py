"""Tests for pyorla.backend."""

from pyorla.backend import new_ollama_backend, new_sglang_backend, new_vllm_backend


def test_new_vllm_backend():
    b = new_vllm_backend("model-a", "http://vllm:8000/v1")
    assert b.endpoint == "http://vllm:8000/v1"
    assert b.type == "openai"
    assert "model-a" in b.model_id
    assert b.name  # non-empty random name


def test_new_sglang_backend():
    b = new_sglang_backend("model-b", "http://sglang:30000/v1")
    assert b.type == "sglang"
    assert "model-b" in b.model_id


def test_new_ollama_backend_appends_v1():
    b = new_ollama_backend("qwen3:0.6b", "http://ollama:11434")
    assert b.endpoint == "http://ollama:11434/v1"
    assert b.type == "openai"


def test_new_ollama_backend_strips_trailing_slash():
    b = new_ollama_backend("qwen3:0.6b", "http://ollama:11434/")
    assert b.endpoint == "http://ollama:11434/v1"
