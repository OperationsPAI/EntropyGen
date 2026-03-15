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

# Install platform tools
RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install kubectl
RUN curl -fsSL "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

# Install openclaw
RUN curl -fsSL https://openclaw.ai/install.sh | bash -s -- --no-prompt --no-onboard

# Copy gitea-cli binary
COPY --from=gitea-cli-builder /app/gitea /usr/local/bin/gitea

# Copy observer binary
COPY --from=observer-builder /app/agent-observer /usr/local/bin/agent-observer

# Copy entrypoint
COPY agent-runtime/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create non-root user for security
RUN useradd -m -u 1000 -s /bin/bash node \
    && mkdir -p /home/node/.openclaw \
    && chown -R node:node /home/node/.openclaw
USER node

# Copy skills (owned by non-root user)
COPY --chown=node:node agent-runtime/skills/ /home/node/.openclaw/skills/

# Copy workspace templates for overriding openclaw defaults
COPY --chown=node:node agent-runtime/builtin-role/workspace-templates/ /agent/workspace-templates/

ENTRYPOINT ["/entrypoint.sh"]
