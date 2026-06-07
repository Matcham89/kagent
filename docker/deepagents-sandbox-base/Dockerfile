# Deep Agents (deepagents-code / dcode) OpenShell sandbox base image.
#
# Ships the dcode interactive TUI, the deepagents SDK, langgraph-cli, and a baked langgraph
# gateway app under /opt/deepagents. The kagent OpenShell harness backend
# (go/core/pkg/sandboxbackend/openshell/deepagents.go) starts that gateway with `langgraph dev`
# after the sandbox is Ready, and SSH sessions launch `dcode`. Model inference routes through the
# OpenShell inference gateway (inference.local) via OPENAI_BASE_URL / OPENAI_API_KEY, mirroring the
# Hermes harness.
FROM python:3.12-slim

# Keep aligned with libs/code (deepagents-code) and libs/deepagents versions upstream.
ARG DEEPAGENTS_CODE_VERSION=0.1.10
ARG DEEPAGENTS_VERSION=0.6.8

ENV PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PATH="/opt/deepagents/.venv/bin:${PATH}"

# Runtime tooling: node (deepagents tools), socat (gateway port-forward), git/curl,
# iproute2 (ss, used by the controller's gateway readiness probe), ca-certificates.
RUN apt-get update && apt-get install -y --no-install-recommends \
        socat git curl ca-certificates iproute2 nodejs npm \
    && rm -rf /var/lib/apt/lists/*

# Non-root sandbox user/group expected by the OpenShell process policy (RunAsUser sandbox).
RUN groupadd -g 1000 sandbox \
    && useradd -m -u 1000 -g sandbox -s /bin/bash sandbox \
    && mkdir -p /sandbox /opt/deepagents \
    && chown -R sandbox:sandbox /sandbox /opt/deepagents

# Install the deepagents SDK, dcode, langgraph-cli, and the OpenAI provider (inference.local route)
# into a venv at /opt/deepagents/.venv, then expose the CLIs on PATH at the paths the network policy
# allowlists (/usr/local/bin/{dcode,langgraph}).
RUN python -m venv /opt/deepagents/.venv \
    && /opt/deepagents/.venv/bin/pip install --no-cache-dir \
        "deepagents==${DEEPAGENTS_VERSION}" \
        "deepagents-code==${DEEPAGENTS_CODE_VERSION}" \
        "langgraph-cli[inmem]" \
        "langchain-openai" \
    && ln -sf /opt/deepagents/.venv/bin/dcode /usr/local/bin/dcode \
    && ln -sf /opt/deepagents/.venv/bin/deepagents-code /usr/local/bin/deepagents-code \
    && ln -sf /opt/deepagents/.venv/bin/langgraph /usr/local/bin/langgraph

# Baked gateway app: langgraph dev serves this; the model is selected via the DEEPAGENTS_MODEL env
# var written by bootstrap into ~/.deepagents/.env.
COPY server_graph.py /opt/deepagents/server_graph.py
COPY langgraph.json /opt/deepagents/langgraph.json
COPY pyproject.toml /opt/deepagents/pyproject.toml
RUN chown -R sandbox:sandbox /opt/deepagents

USER sandbox
WORKDIR /sandbox
ENV HOME=/sandbox

# OpenShell keeps the sandbox VM alive; this is a sensible default entrypoint for a base image.
CMD ["sleep", "infinity"]
