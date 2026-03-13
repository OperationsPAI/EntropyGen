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
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "openai/gpt-4o"}

	data := buildConfigMapData(agent, nil, "http://agent-gateway.test.svc:80", "https://llm.example.com/v1", "sk-test-key")

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
	agents := cfg["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	model := defaults["model"].(map[string]interface{})
	if model["primary"] != "openai/gpt-4o" {
		t.Errorf("default model: got %v, want openai/gpt-4o", model["primary"])
	}
}

func TestBuildConfigMapData_CustomModel(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Soul = "soul"
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
			data := buildSkillsData(agent, nil)

			_, hasGitOps := data[skillKey("git-ops/SKILL.md")]
			_, hasKubectl := data[skillKey("kubectl-ops/SKILL.md")]
			_, hasGiteaAPI := data[skillKey("gitea-api/SKILL.md")]

			if !hasGiteaAPI {
				t.Error("all roles should have gitea-api skill")
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

func TestParseRoleData_WellKnownFiles(t *testing.T) {
	data := map[string]string{
		"SOUL.md":                     "soul content upper",
		"PROMPT.md":                   "prompt content upper",
		"AGENTS.md":                   "agents content upper",
		"skills__my-skill__SKILL.md":  "custom skill",
		"CUSTOM.yaml":                 "custom file",
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

func TestBuildConfigMapData_WithRoleData(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Soul = "spec soul"
	agent.Spec.Role = "developer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "openai/gpt-4o"}

	rd := &roleData{
		Soul:     "role soul content",
		Prompt:   "role prompt content",
		AgentsMD: "# Custom Agents\nCustom role agents.",
	}

	data := buildConfigMapData(agent, rd, "http://gw:80", "https://llm.example.com/v1", "sk-key")

	// roleData.Soul should override spec.soul
	if data["SOUL.md"] != "role soul content" {
		t.Errorf("SOUL.md: got %q, want %q", data["SOUL.md"], "role soul content")
	}
	// roleData.AgentsMD should override template
	if data["AGENTS.md"] != "# Custom Agents\nCustom role agents." {
		t.Errorf("AGENTS.md: got %q, want custom agents", data["AGENTS.md"])
	}
	// roleData.Prompt should be used when cron prompt is empty
	var cronCfg map[string]interface{}
	json.Unmarshal([]byte(data["cron-config.json"]), &cronCfg)
	if cronCfg["prompt"] != "role prompt content" {
		t.Errorf("cron prompt: got %v, want %q", cronCfg["prompt"], "role prompt content")
	}
}

func TestBuildConfigMapData_RoleDataNil(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Soul = "spec soul"
	agent.Spec.Role = "developer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "openai/gpt-4o"}

	data := buildConfigMapData(agent, nil, "http://gw:80", "https://llm.example.com/v1", "sk-key")

	// Without roleData, spec.soul is used
	if data["SOUL.md"] != "spec soul" {
		t.Errorf("SOUL.md: got %q, want %q", data["SOUL.md"], "spec soul")
	}
	// Without roleData, template is used
	if !strings.Contains(data["AGENTS.md"], "Developer Role") {
		t.Errorf("AGENTS.md should contain template content")
	}
}

func TestBuildConfigMapData_CronPromptPriority(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"
	agent.Spec.LLM = &agentapi.AgentLLM{Model: "openai/gpt-4o"}
	agent.Spec.Cron = &agentapi.AgentCron{
		Schedule: "*/5 * * * *",
		Prompt:   "spec cron prompt",
	}

	rd := &roleData{Prompt: "role prompt content"}

	data := buildConfigMapData(agent, rd, "http://gw:80", "https://llm.example.com/v1", "sk-key")

	// spec.cron.prompt should take priority over roleData.Prompt
	var cronCfg map[string]interface{}
	json.Unmarshal([]byte(data["cron-config.json"]), &cronCfg)
	if cronCfg["prompt"] != "spec cron prompt" {
		t.Errorf("cron prompt: got %v, want %q", cronCfg["prompt"], "spec cron prompt")
	}
}

func TestBuildSkillsData_MergeRoleSkills(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	rd := &roleData{
		Skills: map[string]string{
			skillKey("my-custom/SKILL.md"): "# Custom Skill\nDo custom things.\n",
		},
	}

	data := buildSkillsData(agent, rd)

	// Builtin should still exist
	if _, ok := data[skillKey("gitea-api/SKILL.md")]; !ok {
		t.Error("missing builtin gitea-api skill")
	}
	if _, ok := data[skillKey("git-ops/SKILL.md")]; !ok {
		t.Error("missing builtin git-ops skill")
	}
	// Custom skill should be merged
	if _, ok := data[skillKey("my-custom/SKILL.md")]; !ok {
		t.Error("missing custom skill from roleData")
	}
}

func TestBuildSkillsData_NoOverrideBuiltin(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	builtinContent := "# Gitea API Skill\nUse the Gitea REST API to manage issues, PRs, and repositories.\n"
	rd := &roleData{
		Skills: map[string]string{
			skillKey("gitea-api/SKILL.md"): "# Overridden Gitea Skill\nShould not replace builtin.\n",
		},
	}

	data := buildSkillsData(agent, rd)

	if data[skillKey("gitea-api/SKILL.md")] != builtinContent {
		t.Errorf("builtin skill was overridden: got %q", data[skillKey("gitea-api/SKILL.md")])
	}
}
