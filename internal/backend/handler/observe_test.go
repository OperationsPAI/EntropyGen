package handler_test

import (
	"testing"

	"github.com/entropyGen/entropyGen/internal/backend/handler"
)

func TestSidecarAddr(t *testing.T) {
	tests := []struct {
		name      string
		agent     string
		namespace string
		want      string
	}{
		{
			name:      "default namespace",
			agent:     "researcher",
			namespace: "agents",
			want:      "agent-researcher.agents.svc:8081",
		},
		{
			name:      "custom namespace",
			agent:     "coder-01",
			namespace: "custom-ns",
			want:      "agent-coder-01.custom-ns.svc:8081",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.SidecarAddr(tt.agent, tt.namespace)
			if got != tt.want {
				t.Errorf("SidecarAddr(%q, %q) = %q, want %q", tt.agent, tt.namespace, got, tt.want)
			}
		})
	}
}

func TestSidecarPath(t *testing.T) {
	tests := []struct {
		name     string
		fullPath string
		agent    string
		want     string
	}{
		{
			name:     "ws live path",
			fullPath: "/api/agents/researcher/observe/ws/live",
			agent:    "researcher",
			want:     "/ws/live",
		},
		{
			name:     "health endpoint",
			fullPath: "/api/agents/coder-01/observe/healthz",
			agent:    "coder-01",
			want:     "/healthz",
		},
		{
			name:     "root observe path",
			fullPath: "/api/agents/researcher/observe/",
			agent:    "researcher",
			want:     "/",
		},
		{
			name:     "nested path",
			fullPath: "/api/agents/dev/observe/api/v1/status",
			agent:    "dev",
			want:     "/api/v1/status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.SidecarPath(tt.fullPath, tt.agent)
			if got != tt.want {
				t.Errorf("SidecarPath(%q, %q) = %q, want %q", tt.fullPath, tt.agent, got, tt.want)
			}
		})
	}
}
