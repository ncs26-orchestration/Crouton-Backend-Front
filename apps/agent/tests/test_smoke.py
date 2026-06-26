"""Smoke tests — pure, no network, no DB. Proves the package imports and the
JSON-fence helper behaves, so the CI `pytest` step is meaningful from day one.
Feature-specific agent tests (Pydantic AI TestModel/FunctionModel) land with the
agent PRD (see ../../.agents/PRD-AGENT.md, AG-9)."""

from app.nodes.extract import _strip_code_fences


def test_plain_json_passthrough() -> None:
    assert _strip_code_fences('{"a": 1}') == '{"a": 1}'


def test_strips_json_fence() -> None:
    assert _strip_code_fences('```json\n{"a": 1}\n```') == '{"a": 1}'


def test_strips_bare_fence() -> None:
    assert _strip_code_fences('```\n{"a": 1}\n```') == '{"a": 1}'


def test_whitespace_trimmed() -> None:
    assert _strip_code_fences('  {"a": 1}  ') == '{"a": 1}'
