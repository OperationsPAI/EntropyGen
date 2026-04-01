FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.cn,direct go mod download
COPY cmd/backend/ cmd/backend/
COPY internal/backend/ internal/backend/
COPY internal/common/ internal/common/
COPY internal/operator/api/ internal/operator/api/
# Copy builtin templates and skills for Go embed in backend/builtin package
COPY agent-runtimes/openclaw/builtin-role/ internal/backend/builtin/builtin-role/
COPY agent-runtimes/openclaw/skills/ internal/backend/builtin/skills/
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-p=8 go build -o backend ./cmd/backend/

FROM debian:trixie
RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/backend /backend
RUN adduser --disabled-password --gecos "" --no-create-home appuser
USER appuser
ENTRYPOINT ["/backend"]
