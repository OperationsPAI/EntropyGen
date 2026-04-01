package observer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestValidatePath_AllowsNormalPaths(t *testing.T) {
	base := t.TempDir()
	os.MkdirAll(filepath.Join(base, "workspace"), 0o755)
	os.WriteFile(filepath.Join(base, "workspace", "main.go"), []byte("package main"), 0o644)

	if err := ValidatePath(base, "workspace/main.go"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidatePath_BlocksTraversal(t *testing.T) {
	base := t.TempDir()

	cases := []string{
		"../../../etc/passwd",
		"workspace/../../etc/shadow",
		"../secret",
	}

	for _, path := range cases {
		if err := ValidatePath(base, path); err == nil {
			t.Errorf("expected error for path %q, got nil", path)
		}
	}
}

func TestValidatePath_BlocksAbsolutePath(t *testing.T) {
	base := t.TempDir()
	// An absolute path joined with filepath.Join just overwrites
	if err := ValidatePath(base, "/etc/passwd"); err == nil {
		t.Error("expected error for absolute path, got nil")
	}
}

func TestBuildFileTree(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0o644)
	// Hidden dir should be skipped
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(""), 0o644)

	tree, err := BuildFileTree(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tree.Type != "dir" {
		t.Errorf("expected root type dir, got %s", tree.Type)
	}

	// Should have src/ and README.md but not .git/
	names := make(map[string]bool)
	for _, child := range tree.Children {
		names[child.Name] = true
	}
	if names[".git"] {
		t.Error("expected .git to be excluded from tree")
	}
	if !names["src"] {
		t.Error("expected src/ to be in tree")
	}
	if !names["README.md"] {
		t.Error("expected README.md to be in tree")
	}
}

func TestInferLanguage(t *testing.T) {
	cases := map[string]string{
		".go":   "go",
		".py":   "python",
		".ts":   "typescript",
		".json": "json",
		".xyz":  "plaintext",
	}
	for ext, want := range cases {
		got := inferLanguage(ext)
		if got != want {
			t.Errorf("inferLanguage(%q) = %q, want %q", ext, got, want)
		}
	}
}

func TestHealthzEndpoint(t *testing.T) {
	cfg := Config{
		Port:         "0",
		WorkspaceDir: t.TempDir(),
	}
	watcher := NewWatcher(cfg.WorkspaceDir)
	wsHub := NewWSHub(watcher)
	srv := NewServer(cfg, wsHub)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	srv.Router().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %s", body["status"])
	}
}

func TestWorkspaceFileEndpoint_PathTraversal(t *testing.T) {
	cfg := Config{
		Port:         "0",
		WorkspaceDir: t.TempDir(),
	}
	watcher := NewWatcher(cfg.WorkspaceDir)
	wsHub := NewWSHub(watcher)
	srv := NewServer(cfg, wsHub)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/workspace/file?path=../../../etc/passwd", nil)
	srv.Router().ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403 for path traversal, got %d", w.Code)
	}
}

func TestWorkspaceFileEndpoint_MissingPath(t *testing.T) {
	cfg := Config{
		Port:         "0",
		WorkspaceDir: t.TempDir(),
	}
	watcher := NewWatcher(cfg.WorkspaceDir)
	wsHub := NewWSHub(watcher)
	srv := NewServer(cfg, wsHub)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/workspace/file", nil)
	srv.Router().ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for missing path, got %d", w.Code)
	}
}
