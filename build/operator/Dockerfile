FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.cn,direct go mod download
COPY cmd/operator/ cmd/operator/
COPY internal/operator/ internal/operator/
COPY internal/common/ internal/common/
COPY agent-runtimes/openclaw/skills/ internal/operator/reconciler/skills/
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-p=8 go build -o operator ./cmd/operator/

FROM debian:trixie
RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/operator /operator
RUN useradd --no-create-home --shell /bin/false appuser
USER appuser
ENTRYPOINT ["/operator"]
