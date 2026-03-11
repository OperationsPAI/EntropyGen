package giteaclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
)

func setupMockGitea(t *testing.T) (*httptest.Server, *giteaclient.Client) {
	t.Helper()
	mux := http.NewServeMux()

	// Version endpoint
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"version": "1.22.0"})
	})

	// Create user endpoint
	mux.HandleFunc("/api/v1/admin/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Check auth header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    1,
			"login": "test-user",
			"email": "test@devops.local",
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, err := giteaclient.New(srv.URL, "test-admin-token")
	if err != nil {
		t.Fatalf("giteaclient.New: %v", err)
	}
	return srv, client
}

func TestGiteaClient_Version(t *testing.T) {
	_, client := setupMockGitea(t)
	ver, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if ver == "" {
		t.Error("expected non-empty version")
	}
}

func TestGiteaClient_CreateUser_AdminTokenPassed(t *testing.T) {
	_, client := setupMockGitea(t)
	err := client.CreateUser(context.Background(), "agent-test", "agent-test@devops.local", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
}
