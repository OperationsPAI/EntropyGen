package reconciler

import (
	"os"
	"path/filepath"
	"testing"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

func TestBuildPromptConfigMapData_RoleDataUsed(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Name = "dev-1"
	agent.Spec.Role = "developer"

	rd := &roleData{
		Soul:     "role soul content",
		Prompt:   "role prompt for {{AGENT_ID}}",
		AgentsMD: "# Custom Agents",
	}

	data := buildPromptConfigMapData(agent, rd)

	if data["SOUL.md"] != "role soul content" {
		t.Errorf("SOUL.md: got %q", data["SOUL.md"])
	}
	if data["AGENTS.md"] != "# Custom Agents" {
		t.Errorf("AGENTS.md: got %q", data["AGENTS.md"])
	}
	if data["PROMPT.md"] != "role prompt for agent-dev-1" {
		t.Errorf("PROMPT.md: got %q", data["PROMPT.md"])
	}
	if _, exists := data["openclaw.json"]; exists {
		t.Error("openclaw.json should not exist")
	}
}

func TestBuildPromptConfigMapData_NilRoleData(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	data := buildPromptConfigMapData(agent, nil)
	if len(data) != 0 {
		t.Errorf("expected empty map, got %d entries", len(data))
	}
}

func TestAgentRuntimeImage_Selection(t *testing.T) {
	t.Run("nil runtime defaults to env var", func(t *testing.T) {
		t.Setenv("AGENT_RUNTIME_IMAGE", "registry.local/custom:v1")
		agent := &agentapi.Agent{}
		got := agentRuntimeImage(agent)
		if got != "registry.local/custom:v1" {
			t.Errorf("got %q, want registry.local/custom:v1", got)
		}
	})
	t.Run("explicit image overrides type", func(t *testing.T) {
		agent := &agentapi.Agent{}
		agent.Spec.Runtime = &agentapi.AgentRuntime{Type: "openclaw", Image: "my-custom:v2"}
		got := agentRuntimeImage(agent)
		if got != "my-custom:v2" {
			t.Errorf("got %q, want my-custom:v2", got)
		}
	})
}

func TestBuildSkillsData_FromRole(t *testing.T) {
	agent := &agentapi.Agent{}
	agent.Spec.Role = "developer"

	rd := &roleData{
		Skills: map[string]string{
			"skills/gitea-api/SKILL.md": "# Gitea API\nContent.",
			"skills/git-ops/SKILL.md":   "# Git Ops\nContent.",
			"skills/my-custom/SKILL.md": "# Custom Skill\nDo custom things.",
		},
	}

	data := buildSkillsData(agent, rd)

	if len(data) != 3 {
		t.Errorf("expected 3 skills, got %d", len(data))
	}
	// Skills should be stored with __ keys in ConfigMap
	if _, ok := data[skillKey("skills/gitea-api/SKILL.md")]; !ok {
		t.Error("missing gitea-api skill")
	}
	if _, ok := data[skillKey("skills/git-ops/SKILL.md")]; !ok {
		t.Error("missing git-ops skill")
	}
	if _, ok := data[skillKey("skills/my-custom/SKILL.md")]; !ok {
		t.Error("missing custom skill")
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

func TestReadRoleDataFromDir_WellKnownFiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "SOUL.md", "soul content upper")
	writeFile(t, dir, "PROMPT.md", "prompt content upper")
	writeFile(t, dir, "AGENTS.md", "agents content upper")
	writeFile(t, dir, "skills/my-skill/SKILL.md", "custom skill")
	writeFile(t, dir, "CUSTOM.yaml", "custom file")
	writeFile(t, dir, ".metadata.json", `{"description":"test","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`)

	rd, err := readRoleDataFromDir(dir)
	if err != nil {
		t.Fatalf("readRoleDataFromDir: %v", err)
	}

	if rd.Soul != "soul content upper" {
		t.Errorf("Soul: got %q, want %q", rd.Soul, "soul content upper")
	}
	if rd.Prompt != "prompt content upper" {
		t.Errorf("Prompt: got %q, want %q", rd.Prompt, "prompt content upper")
	}
	if rd.AgentsMD != "agents content upper" {
		t.Errorf("AgentsMD: got %q, want %q", rd.AgentsMD, "agents content upper")
	}
	if rd.Skills["skills/my-skill/SKILL.md"] != "custom skill" {
		t.Errorf("Skills: missing expected skill entry")
	}
	if rd.ExtraFiles["CUSTOM.yaml"] != "custom file" {
		t.Errorf("ExtraFiles: missing expected extra file")
	}
	// .metadata.json should be excluded
	if _, ok := rd.ExtraFiles[".metadata.json"]; ok {
		t.Error(".metadata.json should be skipped")
	}
}

func TestReadRoleDataFromDir_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "soul.md", "lowercase soul")
	writeFile(t, dir, "prompt.md", "lowercase prompt")
	writeFile(t, dir, "agents.md", "lowercase agents")

	rd, err := readRoleDataFromDir(dir)
	if err != nil {
		t.Fatalf("readRoleDataFromDir: %v", err)
	}

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
			"skills/gitea-api/SKILL.md": "content",
			"skills/git-ops/SKILL.md":   "content",
		},
	}

	items := buildSkillItems(agent, rd)

	if len(items) != 2 {
		t.Errorf("expected 2 skill items, got %d", len(items))
	}
	// Items should be sorted by path
	if len(items) >= 2 {
		if items[0].Path != "skills/git-ops/SKILL.md" {
			t.Errorf("first item path: got %q, want %q", items[0].Path, "skills/git-ops/SKILL.md")
		}
		if items[0].Key != "skills__git-ops__SKILL.md" {
			t.Errorf("first item key: got %q, want %q", items[0].Key, "skills__git-ops__SKILL.md")
		}
		if items[1].Path != "skills/gitea-api/SKILL.md" {
			t.Errorf("second item path: got %q, want %q", items[1].Path, "skills/gitea-api/SKILL.md")
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

func writeFile(t *testing.T, baseDir, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
