# vLLM OpenAI-compatible server for the calculator_agent_vllm example (tool calling enabled).
# Matches the flag style used in deploy/docker-compose.vllm.yaml.
#
# Build:
#   docker build -t orla-calc-vllm .
#
# Run (requires NVIDIA GPU):
#   docker run --gpus all -p 8000:8000 orla-calc-vllm
#
# Use a different Hugging Face model (edit the CMD --model value, or extend this Dockerfile).
#
# run.py builds this image and starts the container with the Docker Python SDK, then orla_runtime().
# From repo pyorla/:  uv run python examples/calculator_agent_vllm/run.py

FROM vllm/vllm-openai:latest

EXPOSE 8000

CMD ["--model", "Qwen/Qwen3-4B-Instruct-2507", "--host", "0.0.0.0", "--port", "8000", "--enable-auto-tool-choice", "--tool-call-parser", "hermes"]
