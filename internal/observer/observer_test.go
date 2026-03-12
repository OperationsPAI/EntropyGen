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

func TestListSessions_Empty(t *testing.T) {
	dir := t.TempDir()

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_ParsesFiles(t *testing.T) {
	dir := t.TempDir()

	// Create two session files
	line1 := `{"sessionId":"abc-123","timestamp":"2026-03-12T10:00:00Z","type":"session_start"}`
	line2 := `{"role":"user","content":"hello"}`
	os.WriteFile(filepath.Join(dir, "abc-123.jsonl"), []byte(line1+"\n"+line2+"\n"), 0o644)

	line3 := `{"sessionId":"def-456","timestamp":"2026-03-12T11:00:00Z","type":"session_start"}`
	os.WriteFile(filepath.Join(dir, "def-456.jsonl"), []byte(line3+"\n"), 0o644)

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Should be sorted by started_at descending
	if sessions[0].ID != "def-456" {
		t.Errorf("expected first session to be def-456, got %s", sessions[0].ID)
	}
	if sessions[0].MessageCount != 1 {
		t.Errorf("expected message count 1, got %d", sessions[0].MessageCount)
	}
	if sessions[1].ID != "abc-123" {
		t.Errorf("expected second session to be abc-123, got %s", sessions[1].ID)
	}
	if sessions[1].MessageCount != 2 {
		t.Errorf("expected message count 2, got %d", sessions[1].MessageCount)
	}

	// Most recently modified should be current
	currentCount := 0
	for _, s := range sessions {
		if s.IsCurrent {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Errorf("expected exactly 1 current session, got %d", currentCount)
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
		Port:           "0",
		OpenClawHome:   t.TempDir(),
		CompletionsDir: t.TempDir(),
		WorkspaceDir:   t.TempDir(),
	}
	watcher := NewWatcher(cfg.CompletionsDir, cfg.WorkspaceDir)
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

func TestSessionsEndpoint(t *testing.T) {
	compDir := t.TempDir()
	line := `{"sessionId":"test-1","timestamp":"2026-03-12T10:00:00Z"}` + "\n"
	os.WriteFile(filepath.Join(compDir, "test-1.jsonl"), []byte(line), 0o644)

	cfg := Config{
		Port:           "0",
		OpenClawHome:   t.TempDir(),
		CompletionsDir: compDir,
		WorkspaceDir:   t.TempDir(),
	}
	watcher := NewWatcher(cfg.CompletionsDir, cfg.WorkspaceDir)
	wsHub := NewWSHub(watcher)
	srv := NewServer(cfg, wsHub)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sessions", nil)
	srv.Router().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var sessions []SessionInfo
	json.Unmarshal(w.Body.Bytes(), &sessions)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "test-1" {
		t.Errorf("expected session id test-1, got %s", sessions[0].ID)
	}
}

func TestWorkspaceFileEndpoint_PathTraversal(t *testing.T) {
	homeDir := t.TempDir()
	cfg := Config{
		Port:           "0",
		OpenClawHome:   homeDir,
		CompletionsDir: t.TempDir(),
		WorkspaceDir:   t.TempDir(),
	}
	watcher := NewWatcher(cfg.CompletionsDir, cfg.WorkspaceDir)
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
		Port:           "0",
		OpenClawHome:   t.TempDir(),
		CompletionsDir: t.TempDir(),
		WorkspaceDir:   t.TempDir(),
	}
	watcher := NewWatcher(cfg.CompletionsDir, cfg.WorkspaceDir)
	wsHub := NewWSHub(watcher)
	srv := NewServer(cfg, wsHub)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/workspace/file", nil)
	srv.Router().ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for missing path, got %d", w.Code)
	}
}
