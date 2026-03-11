//go:build tools
// +build tools

// Package tools imports dependencies needed by the project but not
// yet directly imported by production code. This file ensures they
// remain in go.mod after go mod tidy.
package tools

import (
	_ "code.gitea.io/sdk/gitea"
	_ "github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/gin-gonic/gin"
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/gorilla/websocket"
	_ "github.com/redis/go-redis/v9"
	_ "github.com/spf13/cobra"
	_ "go.uber.org/zap"
	_ "k8s.io/client-go/kubernetes"
)
