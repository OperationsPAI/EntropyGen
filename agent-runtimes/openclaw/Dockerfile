# Stage 1: Build gitea-cli
FROM golang:1.25-alpine AS gitea-cli-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/gitea-cli/ cmd/gitea-cli/
COPY internal/gitea-cli/ internal/gitea-cli/
RUN CGO_ENABLED=0 GOOS=linux go build -o gitea ./cmd/gitea-cli/

# Stage 2: Build observer
FROM golang:1.25-alpine AS observer-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/observer/ cmd/observer/
COPY internal/observer/ internal/observer/
COPY internal/common/ internal/common/
RUN CGO_ENABLED=0 GOOS=linux go build -o agent-observer ./cmd/observer

# Stage 3: Agent Runtime image
FROM debian:bookworm-slim

USER root

RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

RUN curl -fsSL https://openclaw.ai/install.sh | bash -s -- --no-prompt --no-onboard

COPY --from=gitea-cli-builder /app/gitea /usr/local/bin/gitea
COPY --from=observer-builder /app/agent-observer /usr/local/bin/agent-observer

# Common setup script
COPY agent-runtimes/common/setup-git.sh /common/setup-git.sh
RUN chmod +x /common/setup-git.sh

# Entrypoint
COPY agent-runtimes/openclaw/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN useradd -m -u 1000 -s /bin/bash node \
    && mkdir -p /home/node/.openclaw \
    && chown -R node:node /home/node/.openclaw
USER node

COPY --chown=node:node agent-runtime/skills/ /home/node/.openclaw/skills/
COPY --chown=node:node agent-runtime/builtin-role/workspace-templates/ /agent/workspace-templates/

ENTRYPOINT ["/entrypoint.sh"]
