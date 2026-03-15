package reconciler

import (
	"encoding/json"
	"testing"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

func TestBuildConfigMapData_OpencalwModel(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "openai/gpt-4o"}

	data := buildConfigMapData(agent, nil, "http://agent-gateway.test.svc:80", "https://llm.example.com/v1", "sk-test-key")

	// Verify openclaw.json model field
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(data["openclaw.json"]), &cfg); err != nil {
		t.Fatalf("openclaw.json invalid JSON: %v", err)
	}
	agents := cfg["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	model := defaults["model"].(map[string]interface{})
	if model["primary"] != "openai/gpt-4o" {
		t.Errorf("default model: got %v, want openai/gpt-4o", model["primary"])
	}
}

func TestBuildConfigMapData_CustomModel(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "observer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "litellm/MiniMax-M2.5"}

	data := buildConfigMapData(agent, nil, "http://agent-gateway.test.svc:80", "https://llm.example.com/v1", "sk-test-key")

	var cfg map[string]interface{}
	json.Unmarshal([]byte(data["openclaw.json"]), &cfg)
	agents := cfg["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	model := defaults["model"].(map[string]interface{})
	if model["primary"] != "litellm/MiniMax-M2.5" {
		t.Errorf("custom model: got %v, want litellm/MiniMax-M2.5", model["primary"])
	}

	// Verify provider config uses the correct provider name and model ID
	models := cfg["models"].(map[string]interface{})
	providers := models["providers"].(map[string]interface{})
	litellm := providers["litellm"].(map[string]interface{})
	if litellm["baseUrl"] != "https://llm.example.com/v1" {
		t.Errorf("baseUrl: got %v, want https://llm.example.com/v1", litellm["baseUrl"])
	}
	modelList := litellm["models"].([]interface{})
	firstModel := modelList[0].(map[string]interface{})
	if firstModel["id"] != "MiniMax-M2.5" {
		t.Errorf("model id: got %v, want MiniMax-M2.5", firstModel["id"])
	}
}

func TestBuildConfigMapData_RoleDataUsed(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "openai/gpt-4o"}

	rd := &roleData{
		Soul:     "role soul content",
		Prompt:   "role prompt content",
		AgentsMD: "# Custom Agents\nCustom role agents.",
	}

	data := buildConfigMapData(agent, rd, "http://gw:80", "https://llm.example.com/v1", "sk-key")

	// Role data is the single source of truth
	if data["SOUL.md"] != "role soul content" {
		t.Errorf("SOUL.md: got %q, want %q", data["SOUL.md"], "role soul content")
	}
	if data["AGENTS.md"] != "# Custom Agents\nCustom role agents." {
		t.Errorf("AGENTS.md: got %q, want custom agents", data["AGENTS.md"])
	}
	// PROMPT.md is included for observability
	if data["PROMPT.md"] != "role prompt content" {
		t.Errorf("PROMPT.md: got %q, want %q", data["PROMPT.md"], "role prompt content")
	}
}

func TestBuildConfigMapData_NilRoleData(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "openai/gpt-4o"}

	data := buildConfigMapData(agent, nil, "http://gw:80", "https://llm.example.com/v1", "sk-key")

	// Without roleData, content is empty (no fallback to spec or template)
	if data["SOUL.md"] != "" {
		t.Errorf("SOUL.md should be empty without roleData, got %q", data["SOUL.md"])
	}
	if data["AGENTS.md"] != "" {
		t.Errorf("AGENTS.md should be empty without roleData, got %q", data["AGENTS.md"])
	}
	if _, exists := data["PROMPT.md"]; exists {
		t.Errorf("PROMPT.md should not exist without roleData")
	}
}

func TestBuildSkillsData_FromRole(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	rd := &roleData{
		Skills: map[string]string{
			skillKey("gitea-api/SKILL.md"):  "# Gitea API\nContent.",
			skillKey("git-ops/SKILL.md"):    "# Git Ops\nContent.",
			skillKey("my-custom/SKILL.md"):  "# Custom Skill\nDo custom things.",
		},
	}

	data := buildSkillsData(agent, rd)

	if len(data) != 3 {
		t.Errorf("expected 3 skills, got %d", len(data))
	}
	if _, ok := data[skillKey("gitea-api/SKILL.md")]; !ok {
		t.Error("missing gitea-api skill from role")
	}
	if _, ok := data[skillKey("git-ops/SKILL.md")]; !ok {
		t.Error("missing git-ops skill from role")
	}
	if _, ok := data[skillKey("my-custom/SKILL.md")]; !ok {
		t.Error("missing custom skill from role")
	}
}

func TestBuildSkillsData_NilRoleData(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	data := buildSkillsData(agent, nil)

	// No role data = no skills (skills come from Role now)
	if len(data) != 0 {
		t.Errorf("expected 0 skills without roleData, got %d", len(data))
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

func TestParseRoleData_WellKnownFiles(t *testing.T) {
	data := map[string]string{
		"SOUL.md":                    "soul content upper",
		"PROMPT.md":                  "prompt content upper",
		"AGENTS.md":                  "agents content upper",
		"skills__my-skill__SKILL.md": "custom skill",
		"CUSTOM.yaml":               "custom file",
	}
	rd := parseRoleData(data)

	if rd.Soul != "soul content upper" {
		t.Errorf("Soul: got %q, want %q", rd.Soul, "soul content upper")
	}
	if rd.Prompt != "prompt content upper" {
		t.Errorf("Prompt: got %q, want %q", rd.Prompt, "prompt content upper")
	}
	if rd.AgentsMD != "agents content upper" {
		t.Errorf("AgentsMD: got %q, want %q", rd.AgentsMD, "agents content upper")
	}
	if rd.Skills["skills__my-skill__SKILL.md"] != "custom skill" {
		t.Errorf("Skills: missing expected skill entry")
	}
	if rd.ExtraFiles["CUSTOM.yaml"] != "custom file" {
		t.Errorf("ExtraFiles: missing expected extra file")
	}
}

func TestParseRoleData_CaseInsensitive(t *testing.T) {
	data := map[string]string{
		"soul.md":   "lowercase soul",
		"prompt.md": "lowercase prompt",
		"agents.md": "lowercase agents",
	}
	rd := parseRoleData(data)

	if rd.Soul != "lowercase soul" {
		t.Errorf("Soul: got %q, want %q", rd.Soul, "lowercase soul")
	}
	if rd.Prompt != "lowercase prompt" {
		t.Errorf("Prompt: got %q, want %q", rd.Prompt, "lowercase prompt")
	}
	if rd.AgentsMD != "lowercase agents" {
		t.Errorf("AgentsMD: got %q, want %q", rd.AgentsMD, "lowercase agents")
	}
}

func TestBuildSkillItems_FromRole(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	rd := &roleData{
		Skills: map[string]string{
			skillKey("gitea-api/SKILL.md"): "content",
			skillKey("git-ops/SKILL.md"):   "content",
		},
	}

	items := buildSkillItems(agent, rd)

	if len(items) != 2 {
		t.Errorf("expected 2 skill items, got %d", len(items))
	}
	// Items should be sorted
	if len(items) >= 2 {
		if items[0].Path != "git-ops/SKILL.md" {
			t.Errorf("first item path: got %q, want %q", items[0].Path, "git-ops/SKILL.md")
		}
		if items[1].Path != "gitea-api/SKILL.md" {
			t.Errorf("second item path: got %q, want %q", items[1].Path, "gitea-api/SKILL.md")
		}
	}
}

func TestBuildSkillItems_NilRoleData(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	items := buildSkillItems(agent, nil)

	if len(items) != 0 {
		t.Errorf("expected 0 skill items without roleData, got %d", len(items))
	}
}
