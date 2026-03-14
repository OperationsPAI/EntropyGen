package reconciler_test

import (
	"fmt"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/entropyGen/entropyGen/internal/operator/reconciler"
)

func TestIssueAgentJWT(t *testing.T) {
	secret := []byte("test-signing-secret-32-bytes-ok!")
	tokenStr, err := reconciler.IssueAgentJWT("agent-developer-1", "developer", secret)
	if err != nil {
		t.Fatalf("IssueAgentJWT: %v", err)
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !token.Valid {
		t.Fatal("token not valid")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("claims not MapClaims")
	}
	if claims["agent_id"] != "agent-developer-1" {
		t.Errorf("agent_id: got %v, want agent-developer-1", claims["agent_id"])
	}
	if claims["agent_role"] != "developer" {
		t.Errorf("agent_role: got %v, want developer", claims["agent_role"])
	}
	if claims["sub"] != "agent-developer-1" {
		t.Errorf("sub: got %v, want agent-developer-1", claims["sub"])
	}
	// exp=0 means never expires (design spec: operator.md §3)
	expVal, hasExp := claims["exp"]
	if !hasExp {
		t.Error("token should have exp claim set to 0")
	} else if expVal != float64(0) {
		t.Errorf("exp: got %v, want 0", expVal)
	}
}
