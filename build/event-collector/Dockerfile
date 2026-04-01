FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.cn,direct go mod download
COPY cmd/event-collector/ cmd/event-collector/
COPY internal/event-collector/ internal/event-collector/
COPY internal/common/ internal/common/
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-p=8 go build -o event-collector ./cmd/event-collector/

FROM debian:trixie
RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/event-collector /event-collector
RUN adduser --disabled-password --gecos "" --no-create-home appuser
USER appuser
ENTRYPOINT ["/event-collector"]
