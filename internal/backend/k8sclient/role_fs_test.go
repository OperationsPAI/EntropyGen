package k8sclient

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type testBuiltin struct{}

func (b *testBuiltin) ReadSOUL() string                          { return "# Default Soul" }
func (b *testBuiltin) ReadPrompt() string                        { return "# Default Prompt" }
func (b *testBuiltin) ReadPromptForRole(role string) string      { return "# Prompt for " + role }
func (b *testBuiltin) BuildAgentsMD(role string) string          { return "# Agents for " + role }
func (b *testBuiltin) BuiltinSkillsForRole(role string) []string { return []string{"gitea-api"} }
func (b *testBuiltin) ReadSkill(name string) string              { return "# " + name + " Skill" }

func newTestClient(t *testing.T) (*RoleClient, string) {
	t.Helper()
	dir := t.TempDir()
	return NewRoleClient(dir, nil, &testBuiltin{}), dir
}

func TestCreateAndGet(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	role, err := rc.Create(ctx, CreateRoleRequest{
		Name:        "developer",
		Description: "Dev role",
		Role:        "developer",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if role.Name != "developer" {
		t.Errorf("name: got %q, want %q", role.Name, "developer")
	}
	if role.Description != "Dev role" {
		t.Errorf("description: got %q, want %q", role.Description, "Dev role")
	}
	// Should have builtin files: SOUL.md, PROMPT.md, AGENTS.md, skills/gitea-api/SKILL.md
	fileNames := map[string]bool{}
	for _, f := range role.Files {
		fileNames[f.Name] = true
	}
	for _, expected := range []string{"SOUL.md", "PROMPT.md", "AGENTS.md", "skills/gitea-api/SKILL.md"} {
		if !fileNames[expected] {
			t.Errorf("missing expected file %q", expected)
		}
	}
}

func TestCreateAlreadyExists(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	_, err := rc.Create(ctx, CreateRoleRequest{Name: "test", Description: "test"})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = rc.Create(ctx, CreateRoleRequest{Name: "test", Description: "test"})
	if err == nil {
		t.Fatal("expected error for duplicate create")
	}
}

func TestList(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	rc.Create(ctx, CreateRoleRequest{Name: "r1", Description: "Role 1"})
	rc.Create(ctx, CreateRoleRequest{Name: "r2", Description: "Role 2"})

	roles, err := rc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}
}

func TestDelete(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	rc.Create(ctx, CreateRoleRequest{Name: "del-me", Description: "temp"})
	if err := rc.Delete(ctx, "del-me"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	roles, _ := rc.List(ctx)
	if len(roles) != 0 {
		t.Errorf("expected 0 roles after delete, got %d", len(roles))
	}
}

func TestPutFile(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	rc.Create(ctx, CreateRoleRequest{Name: "test-put", Description: "test"})

	f, err := rc.PutFile(ctx, "test-put", "custom/nested/file.md", "hello world")
	if err != nil {
		t.Fatalf("put file: %v", err)
	}
	if f.Content != "hello world" {
		t.Errorf("content: got %q, want %q", f.Content, "hello world")
	}

	got, err := rc.GetFile(ctx, "test-put", "custom/nested/file.md")
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if got.Content != "hello world" {
		t.Errorf("get content: got %q, want %q", got.Content, "hello world")
	}
}

func TestDeleteFile(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	rc.Create(ctx, CreateRoleRequest{Name: "test-del", Description: "test"})
	rc.PutFile(ctx, "test-del", "skills/my-skill/SKILL.md", "content")

	if err := rc.DeleteFile(ctx, "test-del", "skills/my-skill/SKILL.md"); err != nil {
		t.Fatalf("delete file: %v", err)
	}

	// File should be gone
	_, err := rc.GetFile(ctx, "test-del", "skills/my-skill/SKILL.md")
	if err == nil {
		t.Fatal("expected error for deleted file")
	}

	// Empty parent directories should be cleaned up
	if _, err := os.Stat(filepath.Join(rc.basePath, "test-del", "skills", "my-skill")); !os.IsNotExist(err) {
		t.Error("expected empty parent dir to be cleaned up")
	}
}

func TestRenameFile(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	rc.Create(ctx, CreateRoleRequest{Name: "test-rename", Description: "test"})
	rc.PutFile(ctx, "test-rename", "old-name.md", "rename content")

	f, err := rc.RenameFile(ctx, "test-rename", "old-name.md", "new-name.md")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if f.Name != "new-name.md" {
		t.Errorf("name: got %q, want %q", f.Name, "new-name.md")
	}

	// Old file should be gone
	_, err = rc.GetFile(ctx, "test-rename", "old-name.md")
	if err == nil {
		t.Fatal("expected error for old file name")
	}

	// New file should exist
	got, err := rc.GetFile(ctx, "test-rename", "new-name.md")
	if err != nil {
		t.Fatalf("get renamed file: %v", err)
	}
	if got.Content != "rename content" {
		t.Errorf("content: got %q, want %q", got.Content, "rename content")
	}
}

func TestListFiles(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	rc.Create(ctx, CreateRoleRequest{Name: "test-list", Description: "test"})
	rc.PutFile(ctx, "test-list", "SOUL.md", "soul")
	rc.PutFile(ctx, "test-list", "skills/git-ops/SKILL.md", "skill")

	files, err := rc.ListFiles(ctx, "test-list")
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	names := map[string]bool{}
	for _, f := range files {
		names[f.Name] = true
	}
	if !names["SOUL.md"] {
		t.Error("missing SOUL.md")
	}
	if !names["skills/git-ops/SKILL.md"] {
		t.Error("missing skills/git-ops/SKILL.md")
	}
	// .metadata.json should be excluded
	if names[".metadata.json"] {
		t.Error(".metadata.json should be excluded from file listing")
	}
}

func TestValidatePath_Traversal(t *testing.T) {
	if err := validatePath("../etc/passwd"); err == nil {
		t.Error("expected error for path traversal")
	}
	if err := validatePath("/etc/passwd"); err == nil {
		t.Error("expected error for absolute path")
	}
	if err := validatePath("normal/path/file.md"); err != nil {
		t.Errorf("expected no error for normal path, got: %v", err)
	}
}

func TestUpdateDescription(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	rc.Create(ctx, CreateRoleRequest{Name: "test-desc", Description: "old desc"})
	role, err := rc.UpdateDescription(ctx, "test-desc", "new desc")
	if err != nil {
		t.Fatalf("update description: %v", err)
	}
	if role.Description != "new desc" {
		t.Errorf("description: got %q, want %q", role.Description, "new desc")
	}
}

func TestCreateWithExplicitFiles(t *testing.T) {
	rc, _ := newTestClient(t)
	ctx := context.Background()

	role, err := rc.Create(ctx, CreateRoleRequest{
		Name:        "custom",
		Description: "Custom role",
		Role:        "custom",
		Files: map[string]string{
			"SOUL.md":                    "custom soul",
			"skills/my-skill/SKILL.md":   "my skill content",
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	fileMap := map[string]string{}
	for _, f := range role.Files {
		fileMap[f.Name] = f.Content
	}
	if fileMap["SOUL.md"] != "custom soul" {
		t.Errorf("SOUL.md: got %q, want %q", fileMap["SOUL.md"], "custom soul")
	}
	if fileMap["skills/my-skill/SKILL.md"] != "my skill content" {
		t.Errorf("skill: got %q, want %q", fileMap["skills/my-skill/SKILL.md"], "my skill content")
	}
}
