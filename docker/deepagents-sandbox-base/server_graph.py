"""Baked langgraph gateway graph for the kagent Deep Agents harness.

Exposes a deepagents agent over the ``langgraph dev`` HTTP API (the harness gateway). The model is
selected via the ``DEEPAGENTS_MODEL`` env var (e.g. ``"openai:claude-opus-4-7"``) written by the
controller bootstrap into ``~/.deepagents/.env``; inference routes through the OpenShell inference
gateway via ``OPENAI_BASE_URL`` / ``OPENAI_API_KEY``, mirroring the Hermes harness.

The graph name ``agent`` matches dcode's default assistant id so the interactive TUI and the served
gateway address the same agent.
"""

from __future__ import annotations

import os

from deepagents import create_deep_agent

# init_chat_model (used by create_deep_agent for string specs) honors OPENAI_BASE_URL /
# OPENAI_API_KEY, so the "openai:<model>" spec routes through inference.local.
_MODEL = os.environ.get("DEEPAGENTS_MODEL", "openai:gpt-4o")

graph = create_deep_agent(model=_MODEL)
