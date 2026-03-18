package builtin

import (
	"strings"
	"testing"
)

func TestParseFrontmatter_Valid(t *testing.T) {
	content := []byte("---\nlabel: Developer\ndescription: Build things\n---\n\n## Developer Role\n")
	yml, body := parseFrontmatter(content)
	if yml == nil {
		t.Fatal("expected yaml bytes, got nil")
	}
	if !strings.Contains(string(yml), "label: Developer") {
		t.Errorf("yaml missing label: %s", yml)
	}
	if !strings.Contains(string(body), "## Developer Role") {
		t.Errorf("body missing heading: %s", body)
	}
}

func TestParseFrontmatter_None(t *testing.T) {
	content := []byte("## No Frontmatter\nJust markdown.\n")
	yml, body := parseFrontmatter(content)
	if yml != nil {
		t.Errorf("expected nil yaml, got: %s", yml)
	}
	if string(body) != string(content) {
		t.Errorf("body should equal original content")
	}
}

func TestParseFrontmatter_Malformed(t *testing.T) {
	content := []byte("---\nlabel: Developer\n## no closing delimiter\n")
	yml, body := parseFrontmatter(content)
	if yml != nil {
		t.Errorf("expected nil yaml for malformed, got: %s", yml)
	}
	if string(body) != string(content) {
		t.Errorf("body should equal original content for malformed")
	}
}

func TestListRoleTypes(t *testing.T) {
	types := ListRoleTypes()
	if len(types) < 5 {
		t.Fatalf("expected at least 5 role types, got %d", len(types))
	}

	found := map[string]bool{}
	for _, rt := range types {
		found[rt.Name] = true
		if rt.Label == "" {
			t.Errorf("role %s has empty label", rt.Name)
		}
		if rt.Description == "" {
			t.Errorf("role %s has empty description", rt.Name)
		}
		if len(rt.Skills) == 0 {
			t.Errorf("role %s has no skills", rt.Name)
		}
		if len(rt.Permissions) == 0 {
			t.Errorf("role %s has no permissions", rt.Name)
		}
	}

	for _, expected := range []string{"developer", "reviewer", "sre", "observer", "pm"} {
		if !found[expected] {
			t.Errorf("missing role type: %s", expected)
		}
	}
}

func TestBuiltinSkillsForRole(t *testing.T) {
	tests := []struct {
		role     string
		expected []string
	}{
		{"developer", []string{"gitea-api", "git-ops"}},
		{"sre", []string{"gitea-api", "git-ops", "kubectl-ops"}},
		{"reviewer", []string{"gitea-api"}},
		{"observer", []string{"gitea-api"}},
		{"pm", []string{"gitea-api", "git-ops"}},
	}

	for _, tc := range tests {
		t.Run(tc.role, func(t *testing.T) {
			skills := BuiltinSkillsForRole(tc.role)
			if len(skills) != len(tc.expected) {
				t.Fatalf("role %s: expected %d skills, got %d: %v", tc.role, len(tc.expected), len(skills), skills)
			}
			for i, s := range tc.expected {
				if skills[i] != s {
					t.Errorf("role %s skill[%d]: expected %q, got %q", tc.role, i, s, skills[i])
				}
			}
		})
	}
}

func TestBuildAgentsMD_NoFrontmatter(t *testing.T) {
	for _, role := range []string{"developer", "reviewer", "sre", "observer", "pm"} {
		t.Run(role, func(t *testing.T) {
			md := BuildAgentsMD(role)
			if md == "" {
				t.Fatalf("BuildAgentsMD(%q) returned empty", role)
			}
			// The output must not start with or contain the YAML frontmatter block
			if strings.HasPrefix(strings.TrimSpace(md), "---") {
				t.Errorf("BuildAgentsMD(%q) output starts with YAML frontmatter delimiter", role)
			}
			// Check there's no YAML frontmatter block pattern (---\n...\n---)
			if strings.Contains(md, "skills:\n") {
				t.Errorf("BuildAgentsMD(%q) output contains YAML key 'skills:'", role)
			}
			if strings.Contains(md, "permissions:\n") {
				t.Errorf("BuildAgentsMD(%q) output contains YAML key 'permissions:'", role)
			}
		})
	}
}

func TestBuildAgentsMD_ContainsBaseAndRole(t *testing.T) {
	md := BuildAgentsMD("developer")
	if !strings.Contains(md, "Agent Behavior Constraints") {
		t.Error("BuildAgentsMD should contain base.md content")
	}
	if !strings.Contains(md, "Developer Role") {
		t.Error("BuildAgentsMD should contain developer role content")
	}
}

func TestBuildAgentsMD_UnknownRole(t *testing.T) {
	md := BuildAgentsMD("nonexistent")
	if md == "" {
		t.Fatal("BuildAgentsMD for unknown role should return base content")
	}
	if !strings.Contains(md, "Agent Behavior Constraints") {
		t.Error("BuildAgentsMD for unknown role should return base content")
	}
}
