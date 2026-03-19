"""Tests for pyorla.memory."""

from pyorla.memory import (
    CacheEvent,
    DefaultMemoryPolicy,
    FlushAtBoundaryPolicy,
    PreserveOnSmallIncrementPolicy,
)
from pyorla.types import CACHE_POLICY_AUTO, CACHE_POLICY_FLUSH, CACHE_POLICY_PRESERVE


def test_preserve_on_small_increment():
    p = PreserveOnSmallIncrementPolicy(threshold_tokens=256)
    event = CacheEvent(
        prev_stage_backend="b1",
        prev_stage_model="m1",
        next_stage_backend="b1",
        next_stage_model="m1",
        delta_tokens=100,
    )
    assert p.decide(event) == CACHE_POLICY_PRESERVE


def test_preserve_returns_auto_on_large_delta():
    p = PreserveOnSmallIncrementPolicy(threshold_tokens=256)
    event = CacheEvent(
        prev_stage_backend="b1",
        prev_stage_model="m1",
        next_stage_backend="b1",
        next_stage_model="m1",
        delta_tokens=500,
    )
    assert p.decide(event) == CACHE_POLICY_AUTO


def test_preserve_returns_auto_on_backend_switch():
    p = PreserveOnSmallIncrementPolicy(threshold_tokens=256)
    event = CacheEvent(
        prev_stage_backend="b1",
        prev_stage_model="m1",
        next_stage_backend="b2",
        next_stage_model="m1",
        delta_tokens=10,
    )
    assert p.decide(event) == CACHE_POLICY_AUTO


def test_flush_at_boundary_workflow_complete():
    p = FlushAtBoundaryPolicy()
    event = CacheEvent(transition_type="workflow_complete")
    assert p.decide(event) == CACHE_POLICY_FLUSH


def test_flush_at_boundary_backend_switch():
    p = FlushAtBoundaryPolicy()
    event = CacheEvent(
        prev_stage_backend="b1",
        next_stage_backend="b2",
    )
    assert p.decide(event) == CACHE_POLICY_FLUSH


def test_flush_at_boundary_same_backend():
    p = FlushAtBoundaryPolicy()
    event = CacheEvent(
        prev_stage_backend="b1",
        prev_stage_model="m1",
        next_stage_backend="b1",
        next_stage_model="m1",
    )
    assert p.decide(event) == CACHE_POLICY_AUTO


def test_default_policy_small_increment_same_backend():
    p = DefaultMemoryPolicy(preserve_threshold=256)
    event = CacheEvent(
        prev_stage_backend="b1",
        prev_stage_model="m1",
        next_stage_backend="b1",
        next_stage_model="m1",
        delta_tokens=50,
    )
    assert p.decide(event) == CACHE_POLICY_PRESERVE


def test_default_policy_workflow_complete_different_backend():
    """When backend changes at workflow_complete, flush-at-boundary triggers."""
    p = DefaultMemoryPolicy()
    event = CacheEvent(
        prev_stage_backend="b1",
        prev_stage_model="m1",
        next_stage_backend="b2",
        next_stage_model="m2",
        transition_type="workflow_complete",
    )
    assert p.decide(event) == CACHE_POLICY_FLUSH


def test_default_policy_same_backend_small_delta_preserves():
    """Same backend + small delta -> preserve (first policy wins)."""
    p = DefaultMemoryPolicy()
    event = CacheEvent(
        prev_stage_backend="b1",
        prev_stage_model="m1",
        next_stage_backend="b1",
        next_stage_model="m1",
        delta_tokens=10,
        transition_type="workflow_complete",
    )
    assert p.decide(event) == CACHE_POLICY_PRESERVE
