package reconciler

import (
	"encoding/json"
	"strings"
	"testing"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

func TestBuildConfigMapData_SoulContent(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Soul = "You are a test agent."
	agent.Spec.Role = "developer"

	data := buildConfigMapData(agent)

	if data["SOUL.md"] != "You are a test agent." {
		t.Errorf("SOUL.md: got %q, want %q", data["SOUL.md"], "You are a test agent.")
	}
	if !strings.Contains(data["AGENTS.md"], "Developer Role") {
		t.Errorf("AGENTS.md missing 'Developer Role', got: %s", data["AGENTS.md"])
	}

	// Verify openclaw.json model field
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(data["openclaw.json"]), &cfg); err != nil {
		t.Fatalf("openclaw.json invalid JSON: %v", err)
	}
	agentSection, _ := cfg["agent"].(map[string]interface{})
	if agentSection["model"] != "gpt-4o" {
		t.Errorf("default model: got %v, want gpt-4o", agentSection["model"])
	}
}

func TestBuildConfigMapData_CustomModel(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Soul = "soul"
	agent.Spec.Role = "observer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "claude-3-5-sonnet"}

	data := buildConfigMapData(agent)

	var cfg map[string]interface{}
	json.Unmarshal([]byte(data["openclaw.json"]), &cfg)
	agentSection := cfg["agent"].(map[string]interface{})
	if agentSection["model"] != "claude-3-5-sonnet" {
		t.Errorf("custom model: got %v, want claude-3-5-sonnet", agentSection["model"])
	}
}

func TestBuildSkillsData_Roles(t *testing.T) {
	tests := []struct {
		role          string
		expectGitOps  bool
		expectKubectl bool
	}{
		{"observer", false, false},
		{"developer", true, false},
		{"reviewer", false, false},
		{"sre", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			agent := &agentapi.Agent{}
			agent.Spec.Role = tt.role
			data := buildSkillsData(agent)

			_, hasGitOps := data["git-ops/SKILL.md"]
			_, hasKubectl := data["kubectl-ops/SKILL.md"]
			_, hasGiteaAPI := data["gitea-api/SKILL.md"]

			if !hasGiteaAPI {
				t.Error("all roles should have gitea-api/SKILL.md")
			}
			if hasGitOps != tt.expectGitOps {
				t.Errorf("git-ops: got %v, want %v", hasGitOps, tt.expectGitOps)
			}
			if hasKubectl != tt.expectKubectl {
				t.Errorf("kubectl-ops: got %v, want %v", hasKubectl, tt.expectKubectl)
			}
		})
	}
}

func TestComputeHash_Deterministic(t *testing.T) {
	data := map[string]string{"a": "val1", "b": "val2"}
	h1 := computeHash(data)
	h2 := computeHash(data)
	if h1 != h2 {
		t.Error("hash not deterministic")
	}
	if len(h1) != 16 {
		t.Errorf("hash length: got %d, want 16", len(h1))
	}
}
