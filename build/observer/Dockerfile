FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/observer/ cmd/observer/
COPY internal/observer/ internal/observer/
RUN CGO_ENABLED=0 GOOS=linux go build -o /agent-observer ./cmd/observer

FROM alpine:3.21
RUN apk add --no-cache git
COPY --from=builder /agent-observer /usr/local/bin/agent-observer
EXPOSE 8081
ENTRYPOINT ["agent-observer"]
