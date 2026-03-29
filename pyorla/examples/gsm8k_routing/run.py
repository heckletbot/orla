"""GSM8K evaluation with LangGraph + Orla accuracy routing.

Follows the standard GSM8K evaluation protocol (5-shot, ``#### <number>``
extraction, exact-match scoring) so results are comparable to published
leaderboard numbers.

Three modes compare cost vs accuracy:

- **baseline**: every problem routes to the strong model (high accuracy, high cost).
- **all-cheap**: every problem routes to the cheap model (low cost, lower accuracy).
- **routed** (default): a triage node uses the cheap model to classify difficulty,
  then sets ``accuracy`` so Orla picks the cheapest qualifying backend under
  ``ACCURACY_POLICY_PREFER``.

Running all three modes on the same slice gives three points on the
cost–accuracy Pareto frontier.  Orla's per-request ``estimated_cost_usd``
is accumulated and reported in the summary.

Model tiers (all open-weight, Bedrock Mantle defaults, overridable via env vars):

- **Cheap**: Ministral 3B — very low cost, handles easy arithmetic (single consumer GPU).
- **Mid**: Qwen3 32B dense — strong math reasoning at moderate cost (single high-end GPU).
- **Strong**: Qwen3 235B A22B — frontier-class MoE reasoning (22B active params).

All models are open-weight and self-hostable via vLLM or Ollama.  Override
``GSM8K_CHEAP_MODEL``, ``GSM8K_MID_MODEL``, ``GSM8K_STRONG_MODEL``, and
``OPENAI_BASE_URL`` to point at a local vLLM/Ollama endpoint.

Prerequisites:

- ``orla`` on PATH (or ``ORLA_BIN``); this script starts a local daemon via
  ``orla_runtime()``.
- ``OPENAI_API_KEY`` for the Bedrock Mantle OpenAI-compatible API (or for your
  local vLLM/Ollama endpoint).
- Example extras (Hugging Face ``datasets``)::

    uv sync --group examples

Run from the ``pyorla`` directory::

    uv run python examples/gsm8k_routing/run.py --mode routed --limit 20
    uv run python examples/gsm8k_routing/run.py --mode baseline --limit 20
    uv run python examples/gsm8k_routing/run.py --mode all-cheap --limit 20

Use ``--limit 0`` to evaluate the entire split from ``--start`` (standard full
test set: ``--split test --start 0 --limit 0``).

Per-example metrics are written to ``results.csv`` by default (override with
``--output-csv``; empty value disables).  Columns include ``cost_usd``,
``triage_cost_usd``, ``solve_cost_usd``, ``correct``, ``routing_accuracy``, and
``difficulty`` for plotting.
"""

from __future__ import annotations

import argparse
import csv
import os
import re
from dataclasses import dataclass, field
from typing import Annotated, Any

from langchain_core.messages import AIMessage, AnyMessage, HumanMessage, SystemMessage
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
from pyorla.types import ACCURACY_POLICY_PREFER, CostModel

try:
    from datasets import load_dataset
except ImportError as exc:
    raise SystemExit(
        "This example requires Hugging Face `datasets`. Install with:\n"
        "  uv sync --group examples\n"
        "then re-run."
    ) from exc

MANTLE_BASE_URL = "https://bedrock-mantle.us-east-2.api.aws/v1"
DEFAULT_CHEAP_MODEL = "openai:mistral.ministral-3-3b-instruct"
DEFAULT_MID_MODEL = "openai:qwen.qwen3-32b-v1:0"
DEFAULT_STRONG_MODEL = "openai:qwen.qwen3-235b-a22b-2507-v1:0"

NUM_FEWSHOT = 5

TRIAGE_SYSTEM_PROMPT = (
    "You classify grade-school math problems by difficulty.\n"
    "Output ONLY one word: easy, medium, or hard.\n"
    "- easy: single-step arithmetic (addition, subtraction, simple multiplication)\n"
    "- medium: two or three steps, straightforward logic\n"
    "- hard: four+ steps, fractions, percentages, rates, or tricky wording"
)

DIFFICULTY_TO_ACCURACY: dict[str, float] = {
    "easy": 0.30,
    "medium": 0.60,
    "hard": 0.85,
}
DEFAULT_ACCURACY = 0.85


def _env(key: str, default: str) -> str:
    return os.environ.get(key, default).strip()

BASELINE_ACCURACY = 0.90

# ---------------------------------------------------------------------------
# GSM8K dataset helpers
# ---------------------------------------------------------------------------

# After ``extract_answer`` captures a substring (its regexes allow ``$`` and ``,``),
# we strip those here so gold and prediction compare as plain integers/strings.
_NORMALIZE_RE = re.compile(r"[$,]")


def _normalize_number(s: str) -> str:
    """Strip ``$``, commas, trailing period; collapse to bare number string."""
    s = _NORMALIZE_RE.sub("", s).strip().rstrip(".")
    try:
        return str(int(float(s)))
    except (ValueError, OverflowError):
        return s


def _gsm8k_gold(answer_field: str) -> str:
    """Final numeric answer after GSM8K's ``####`` marker."""
    if "####" in answer_field:
        raw = answer_field.rsplit("####", 1)[-1].strip()
    else:
        raw = answer_field.strip()
    return _normalize_number(raw)


def _build_fewshot_prefix(train_ds: Any, n: int) -> str:
    """Build a 5-shot prefix matching lm-evaluation-harness format."""
    lines: list[str] = []
    for i in range(n):
        q = train_ds[i]["question"]
        a = train_ds[i]["answer"]
        lines.append(f"Question: {q}\nAnswer: {a}")
    return "\n\n".join(lines)


def load_gsm8k(
    *, split: str, start: int, limit: int
) -> tuple[list[tuple[str, str]], str]:
    """Load items and build the few-shot prefix from train.

    *limit* is the number of examples after *start*, or ``0`` meaning through
    the end of the split.

    Returns ``(items, fewshot_prefix)`` where each item is ``(question, gold)``.
    """
    train_ds = load_dataset("gsm8k", "main", split="train")
    fewshot_prefix = _build_fewshot_prefix(train_ds, NUM_FEWSHOT)

    ds = load_dataset("gsm8k", "main", split=split)
    n = len(ds)
    if start < 0 or start >= n:
        raise SystemExit(f"--start {start} out of range for split {split!r} (len={n})")
    if limit == 0:
        end = n
    else:
        end = min(start + limit, n)
    items = [(ds[i]["question"], _gsm8k_gold(ds[i]["answer"])) for i in range(start, end)]
    return items, fewshot_prefix


def extract_answer(text: str) -> str | None:
    """Extract final numeric answer from model output.

    Matches the lm-evaluation-harness protocol: strict ``#### <number>``
    first, then flexible last-number fallback.
    """
    strict = re.findall(r"####\s*(\-?[\d,.$]+)", text)
    if strict:
        return _normalize_number(strict[-1])
    flexible = re.findall(r"(-?[$\d,]+\.?\d*)", text)
    if flexible:
        candidate = flexible[-1]
        if any(c.isdigit() for c in candidate):
            return _normalize_number(candidate)
    return None


# ---------------------------------------------------------------------------
# Cost tracking
# ---------------------------------------------------------------------------

def _extract_cost(msg: AIMessage) -> float:
    """Read Orla's ``estimated_cost_usd`` from LangChain response_metadata."""
    meta = getattr(msg, "response_metadata", None) or {}
    return float(meta.get("estimated_cost_usd", 0.0) or 0.0)


def _accuracy_for_log(out: dict[str, Any], mode: str) -> float | None:
    """Accuracy floor used for routing (state may omit it for non-routed modes)."""
    v = out.get("required_accuracy")
    if v is not None:
        return float(v)
    if mode == "baseline":
        return BASELINE_ACCURACY
    if mode == "all-cheap":
        return 0.0
    return None


def _triage_solve_costs(out: dict[str, Any], mode: str) -> tuple[float, float]:
    """Split Orla estimated cost between triage and solve ``AIMessage``s."""
    ai_msgs = [m for m in out["messages"] if isinstance(m, AIMessage)]
    if mode == "routed" and len(ai_msgs) >= 2:
        return _extract_cost(ai_msgs[0]), _extract_cost(ai_msgs[-1])
    if ai_msgs:
        return 0.0, _extract_cost(ai_msgs[-1])
    return 0.0, 0.0


# ---------------------------------------------------------------------------
# LangGraph state
# ---------------------------------------------------------------------------

@dataclass
class GSMState:
    """Messages plus the accuracy floor for Orla routing."""

    messages: Annotated[list[AnyMessage], add_messages] = field(default_factory=list)
    required_accuracy: float | None = None
    difficulty: str = ""


# ---------------------------------------------------------------------------
# Graph builders
# ---------------------------------------------------------------------------

def _user_text(state: GSMState) -> str:
    for m in reversed(state.messages):
        if isinstance(m, HumanMessage) and m.content:
            return str(m.content)
    return ""


def build_graph(
    solve_stage: Stage,
    *,
    mode: str,
    triage_stage: Stage | None = None,
    fewshot_prefix: str = "",
):
    """Build a LangGraph for GSM8K.

    *mode* is one of ``routed``, ``baseline``, ``all-cheap``.
    """
    solve_llm = solve_stage.as_chat_model()

    def triage_node(state: GSMState) -> dict[str, Any]:
        assert triage_stage is not None
        triage_llm = triage_stage.as_chat_model()
        question = _user_text(state)
        reply = triage_llm.invoke([
            SystemMessage(content=TRIAGE_SYSTEM_PROMPT),
            HumanMessage(content=question),
        ])
        label = str(reply.content).strip().lower().rstrip(".")
        acc = DIFFICULTY_TO_ACCURACY.get(label, DEFAULT_ACCURACY)
        return {"messages": [reply], "required_accuracy": acc, "difficulty": label}

    def solve_node(state: GSMState) -> dict[str, Any]:
        if mode == "baseline":
            solve_stage.set_accuracy(BASELINE_ACCURACY)
        elif mode == "all-cheap":
            solve_stage.set_accuracy(0.0)
        else:
            acc = state.required_accuracy if state.required_accuracy is not None else DEFAULT_ACCURACY
            solve_stage.set_accuracy(acc)
        solve_stage.set_accuracy_policy(ACCURACY_POLICY_PREFER)

        question = _user_text(state)
        prompt = f"{fewshot_prefix}\n\nQuestion: {question}\nAnswer:"
        reply = solve_llm.invoke([HumanMessage(content=prompt)])
        return {"messages": [reply]}

    g = StateGraph(GSMState)
    g.add_node("solve", solve_node)

    if mode == "routed":
        g.add_node("triage", triage_node)
        g.add_edge(START, "triage")
        g.add_edge("triage", "solve")
    else:
        g.add_edge(START, "solve")

    g.add_edge("solve", END)
    return g.compile()


# ---------------------------------------------------------------------------
# Backend registration
# ---------------------------------------------------------------------------

def _register_backends(client: OrlaClient) -> tuple[LLMBackend, LLMBackend]:
    """Register cheap / mid / strong backends.

    Returns ``(cheap_backend, triage_backend)`` — cheap is used as the default
    Stage backend name (Orla rewrites it via accuracy routing); triage backend
    is used for the triage Stage.
    """
    api_key_env = "OPENAI_API_KEY"
    cheap = LLMBackend(
        name="gsm-cheap",
        endpoint=_env("OPENAI_BASE_URL", MANTLE_BASE_URL),
        type="openai",
        model_id=_env("GSM8K_CHEAP_MODEL", DEFAULT_CHEAP_MODEL),
        api_key_env_var=api_key_env,
        quality=0.30,
        cost_model=CostModel(
            input_cost_per_mtoken=0.10,
            output_cost_per_mtoken=0.10,
        ),
    )
    mid = LLMBackend(
        name="gsm-mid",
        endpoint=_env("OPENAI_BASE_URL", MANTLE_BASE_URL),
        type="openai",
        model_id=_env("GSM8K_MID_MODEL", DEFAULT_MID_MODEL),
        api_key_env_var=api_key_env,
        quality=0.60,
        cost_model=CostModel(
            input_cost_per_mtoken=0.15,
            output_cost_per_mtoken=0.60,
        ),
    )
    strong = LLMBackend(
        name="gsm-strong",
        endpoint=_env("OPENAI_BASE_URL", MANTLE_BASE_URL),
        type="openai",
        model_id=_env("GSM8K_STRONG_MODEL", DEFAULT_STRONG_MODEL),
        api_key_env_var=api_key_env,
        quality=0.90,
        cost_model=CostModel(
            input_cost_per_mtoken=0.53,
            output_cost_per_mtoken=2.66,
        ),
    )
    for b in (cheap, mid, strong):
        client.register_backend(b)
    return cheap, cheap


# ---------------------------------------------------------------------------
# Run loop and scoring
# ---------------------------------------------------------------------------

def run_benchmark(
    client: OrlaClient,
    items: list[tuple[str, str]],
    fewshot_prefix: str,
    *,
    mode: str,
    split: str,
    start: int,
    output_csv: str | None,
) -> None:
    """Run the LangGraph agent on each (question, gold_answer) pair and score.

    If *output_csv* is set, write one UTF-8 CSV row per example (columns suit
    cost vs accuracy plots). Use an empty ``--output-csv`` argument to skip.
    """
    cheap_be, triage_be = _register_backends(client)

    solve_stage = Stage("gsm8k-solve", cheap_be)
    solve_stage.client = client
    solve_stage.set_temperature(0.0)
    solve_stage.set_max_tokens(768)

    triage_stage: Stage | None = None
    if mode == "routed":
        triage_stage = Stage("gsm8k-triage", triage_be)
        triage_stage.client = client
        triage_stage.set_temperature(0.0)
        triage_stage.set_max_tokens(8)

    agent = build_graph(
        solve_stage,
        mode=mode,
        triage_stage=triage_stage,
        fewshot_prefix=fewshot_prefix,
    )

    correct = 0
    total = len(items)
    total_cost_usd = 0.0
    difficulty_counts: dict[str, int] = {"easy": 0, "medium": 0, "hard": 0, "unknown": 0}

    csv_file = None
    csv_writer: csv.DictWriter | None = None
    if output_csv:
        csv_file = open(output_csv, "w", newline="", encoding="utf-8")
        fieldnames = (
            "split",
            "dataset_index",
            "run_index",
            "mode",
            "gold",
            "predicted",
            "correct",
            "routing_accuracy",
            "difficulty",
            "triage_cost_usd",
            "solve_cost_usd",
            "cost_usd",
            "question",
        )
        csv_writer = csv.DictWriter(csv_file, fieldnames=fieldnames)
        csv_writer.writeheader()

    try:
        for idx, (question, gold) in enumerate(items):
            print(f"\n{'=' * 60}")
            print(f"[{idx + 1}/{total}]  gold={gold}")
            print(f"{'=' * 60}")
            print(question.strip())

            out = agent.invoke({"messages": [HumanMessage(content=question)]})

            last_msg = out["messages"][-1]
            reply_text = str(last_msg.content) if isinstance(last_msg, AIMessage) else str(last_msg)
            predicted = extract_answer(reply_text)
            match = predicted is not None and predicted == gold

            triage_c, solve_c = _triage_solve_costs(out, mode)
            item_cost = triage_c + solve_c
            total_cost_usd += item_cost

            difficulty = out.get("difficulty", "")
            if difficulty:
                difficulty_counts[difficulty] = difficulty_counts.get(difficulty, 0) + 1
            elif mode == "routed":
                difficulty_counts["unknown"] += 1

            status = "CORRECT" if match else "WRONG"
            correct += match
            acc_str = f"acc={out.get('required_accuracy', 'n/a')}"
            diff_str = f"difficulty={difficulty}" if difficulty else ""
            cost_str = f"cost=${item_cost:.6f}" if item_cost > 0 else ""
            print(f"  predicted={predicted}  {status}  {acc_str}  {diff_str}  {cost_str}")

            if csv_writer is not None:
                acc_log = _accuracy_for_log(out, mode)
                csv_writer.writerow(
                    {
                        "split": split,
                        "dataset_index": start + idx,
                        "run_index": idx,
                        "mode": mode,
                        "gold": gold,
                        "predicted": predicted if predicted is not None else "",
                        "correct": 1 if match else 0,
                        "routing_accuracy": f"{acc_log:.4f}" if acc_log is not None else "",
                        "difficulty": difficulty,
                        "triage_cost_usd": f"{triage_c:.8f}",
                        "solve_cost_usd": f"{solve_c:.8f}",
                        "cost_usd": f"{item_cost:.8f}",
                        "question": question.strip(),
                    }
                )

    finally:
        if csv_file is not None:
            csv_file.close()

    pct = 100.0 * correct / total if total else 0.0
    avg_cost = total_cost_usd / total if total else 0.0
    print(f"\n{'=' * 60}")
    print(f"  Mode:       {mode}")
    print(f"  Items:      {total}")
    print(f"  Correct:    {correct}")
    print(f"  Accuracy:   {pct:.1f}%")
    print(f"  Total cost: ${total_cost_usd:.6f}")
    print(f"  Avg cost:   ${avg_cost:.6f} / item")
    if mode == "routed":
        print(f"  Difficulty: {dict(difficulty_counts)}")
    if output_csv:
        print(f"  CSV:        {output_csv}")
    print(f"{'=' * 60}")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(
        description="GSM8K benchmark with LangGraph + Orla accuracy routing.",
    )
    parser.add_argument(
        "--mode",
        choices=("baseline", "all-cheap", "routed"),
        default="routed",
        help=(
            "baseline: always strong model. "
            "all-cheap: always cheap model. "
            "routed: LLM triage → Orla picks cheapest qualifying backend (default)."
        ),
    )
    parser.add_argument(
        "--split",
        choices=("train", "test"),
        default="test",
        help="GSM8K split (default: test).",
    )
    parser.add_argument(
        "--start",
        type=int,
        default=0,
        help="0-based index into the split.",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=5,
        help=(
            "Number of consecutive examples after --start (default: 5). "
            "Use 0 for the rest of the split (full eval: --split test --start 0 --limit 0)."
        ),
    )
    parser.add_argument(
        "--output-csv",
        default="results.csv",
        metavar="PATH",
        help=(
            "Write per-example results for plotting (UTF-8). "
            "Default: results.csv. Use an empty string to disable."
        ),
    )
    args = parser.parse_args()

    if args.limit < 0:
        raise SystemExit("--limit must be >= 0 (0 means through end of split)")
    if args.start < 0:
        raise SystemExit("--start must be >= 0")

    items, fewshot_prefix = load_gsm8k(
        split=args.split, start=args.start, limit=args.limit,
    )

    if not _env("OPENAI_API_KEY", ""):
        raise SystemExit(
            "OPENAI_API_KEY must be set so `orla serve` can call the Mantle endpoint."
        )

    out_csv = (args.output_csv or "").strip()
    csv_path = out_csv if out_csv else None

    try:
        with orla_runtime(quiet=True) as client:
            run_benchmark(
                client,
                items,
                fewshot_prefix,
                mode=args.mode,
                split=args.split,
                start=args.start,
                output_csv=csv_path,
            )
    except OrlaBinaryNotFoundError as exc:
        raise SystemExit(
            f"{exc}\n"
            "Install Orla, put `orla` on PATH, or set ORLA_BIN to the binary path."
        ) from exc
    except OrlaError as exc:
        raise SystemExit(str(exc)) from exc


if __name__ == "__main__":
    main()
