package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/entropyGen/entropyGen/internal/common/models"
	"github.com/entropyGen/entropyGen/internal/gateway/audit"
	"github.com/entropyGen/entropyGen/internal/gateway/gatewayctx"
)

// AuthMiddleware validates JWT Bearer tokens and injects agent identity into context.
type AuthMiddleware struct {
	secret      []byte
	eventWriter *audit.EventWriter
}

// NewAuthMiddleware creates a new AuthMiddleware with the given HMAC secret.
func NewAuthMiddleware(secret []byte, ew *audit.EventWriter) *AuthMiddleware {
	return &AuthMiddleware{secret: secret, eventWriter: ew}
}

// Wrap returns an http.Handler that enforces JWT authentication before calling next.
func (a *AuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := a.verifyToken(r)
		if err != nil {
			a.writeAuthFailedEvent(r, err.Error())
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		agentID, _ := claims["agent_id"].(string)
		agentRole, _ := claims["agent_role"].(string)

		ctx := context.WithValue(r.Context(), gatewayctx.AgentID, agentID)
		ctx = context.WithValue(ctx, gatewayctx.AgentRole, agentRole)
		r = r.WithContext(ctx)
		r.Header.Set("X-Agent-ID", agentID)
		r.Header.Set("X-Agent-Role", agentRole)

		next.ServeHTTP(w, r)
	})
}

func (a *AuthMiddleware) verifyToken(r *http.Request) (*jwt.Token, error) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("missing or invalid Authorization header")
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

	// WithoutClaimsValidation skips exp/nbf/iat time checks.
	// Required because Operator signs tokens with exp=0 (never expires).
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, err := parser.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("token parse error: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}
	return token, nil
}

func (a *AuthMiddleware) writeAuthFailedEvent(r *http.Request, reason string) {
	payload, _ := json.Marshal(map[string]string{
		"trace_id": uuid.New().String(),
		"reason":   reason,
		"path":     r.URL.Path,
		"method":   r.Method,
	})
	a.eventWriter.Enqueue(&models.Event{
		EventID:   uuid.New().String(),
		EventType: "gateway.auth_failed",
		Timestamp: time.Now(),
		Payload:   payload,
	})
}
