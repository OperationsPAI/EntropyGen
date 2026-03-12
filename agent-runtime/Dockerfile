# Stage 1: Build gitea-cli
FROM golang:1.23-alpine AS gitea-cli-builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o gitea ./cmd/gitea-cli/

# Stage 2: Agent Runtime image
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

# Copy gitea-cli binary
COPY --from=gitea-cli-builder /app/gitea /usr/local/bin/gitea

# Copy entrypoint
COPY agent-runtime/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create non-root user for security
RUN useradd -m -u 1000 -s /bin/bash node
USER node

# Copy skills (owned by non-root user)
COPY --chown=node:node agent-runtime/skills/ /home/node/.openclaw/skills/

ENTRYPOINT ["/entrypoint.sh"]
