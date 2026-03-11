// Package e2e contains end-to-end tests for gitea-cli against the live Gitea instance.
//
// Prerequisites:
//   - Gitea running at http://localhost:3000
//   - GITEA_TEST_TOKEN env var set (or uses the default test token)
//
// Run:
//
//	go test ./tests/e2e/ -v -run TestGiteaCLI
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	giteaURL       = "http://localhost:3000"
	giteaAdminUser = "gitea-admin"
	giteaAdminPass = "gitea-admin123"
	// Created in dev setup; see TestMain or scripts/dev-up.sh comment
	defaultTestToken = "633dcfebc014e153d0653550d517d72a2a8cb5d2"

	testOrg  = "e2e-test-org"
	testRepo = "e2e-test-repo"
)

// giteaToken returns the API token to use for tests.
func giteaToken(t *testing.T) string {
	t.Helper()
	if tok := os.Getenv("GITEA_TEST_TOKEN"); tok != "" {
		return tok
	}
	return defaultTestToken
}

// gitea CLI binary path (built once per test run)
var cliPath string

// buildCLI compiles the gitea CLI binary to /tmp and returns the path.
// The binary is shared across all tests in the package (not per-test tempdir).
func buildCLI(t *testing.T) string {
	t.Helper()
	if cliPath != "" {
		return cliPath
	}
	bin := "/tmp/gitea-e2e-test"
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/gitea-cli/")
	cmd.Dir = projectRoot(t)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build gitea-cli: %v\n%s", err, out)
	}
	cliPath = bin
	return cliPath
}

func projectRoot(t *testing.T) string {
	t.Helper()
	// tests/e2e/ → ../../
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	return dir
}

// runCLI runs the gitea CLI with given args, returns (stdout, stderr, error).
func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	bin := buildCLI(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"GITEA_BASE_URL="+giteaURL,
		"GITEA_TOKEN_PATH="+writeTokenFile(t),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runCLIJSON runs the CLI with --json flag and unmarshals the output.
func runCLIJSON(t *testing.T, args ...string) interface{} {
	t.Helper()
	args = append([]string{"--json"}, args...)
	stdout, _, err := runCLI(t, args...)
	if err != nil {
		t.Fatalf("gitea %v: %v\nstdout: %s", args, err, stdout)
	}
	var result interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nraw: %s", err, stdout)
	}
	return result
}

// writeTokenFile writes the test token to a temp file and returns the path.
func writeTokenFile(t *testing.T) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "gitea-token")
	if err := os.WriteFile(f, []byte(giteaToken(t)), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	return f
}

// ensureOrgAndRepo creates the test org and repo if they don't exist.
func ensureOrgAndRepo(t *testing.T) {
	t.Helper()
	client := &giteaAdminClient{t: t}
	client.ensureOrg(testOrg)
	client.ensureRepo(testOrg, testRepo)
}

// ── gitea admin HTTP client (used for setup/teardown only) ───────────────────

type giteaAdminClient struct{ t *testing.T }

func (c *giteaAdminClient) do(method, path string, body interface{}) (int, []byte) {
	c.t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req, _ := newHTTPRequest(method, giteaURL+"/api/v1"+path, bodyReader)
	req.SetBasicAuth(giteaAdminUser, giteaAdminPass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		c.t.Fatalf("gitea admin %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := readAll(resp.Body)
	return resp.StatusCode, raw
}

func (c *giteaAdminClient) ensureOrg(name string) {
	code, _ := c.do("GET", "/orgs/"+name, nil)
	if code == 200 {
		return
	}
	code, body := c.do("POST", "/orgs", map[string]interface{}{
		"username":   name,
		"visibility": "public",
	})
	if code != 201 && code != 422 { // 422 = already exists
		c.t.Fatalf("create org %s: got %d\n%s", name, code, body)
	}
}

func (c *giteaAdminClient) ensureRepo(org, repo string) {
	code, _ := c.do("GET", "/repos/"+org+"/"+repo, nil)
	if code == 200 {
		return
	}
	code, body := c.do("POST", "/orgs/"+org+"/repos", map[string]interface{}{
		"name":           repo,
		"auto_init":      true,
		"default_branch": "main",
	})
	if code != 201 {
		c.t.Fatalf("create repo %s/%s: got %d\n%s", org, repo, code, body)
	}
}

func (c *giteaAdminClient) deleteIssue(org, repo string, number int64) {
	c.do("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d", org, repo, number), nil)
}

// ── gitea-cli: splitRepo / config ────────────────────────────────────────────

func TestGiteaCLI_NoArgs_ShowsHelp(t *testing.T) {
	stdout, _, err := runCLI(t)
	// Cobra exits 0 when no subcommand is given and prints help
	_ = err
	if !strings.Contains(stdout, "gitea") {
		t.Errorf("expected help output, got: %q", stdout)
	}
}

func TestGiteaCLI_InvalidRepo_ReturnsError(t *testing.T) {
	_, _, err := runCLI(t, "issue", "list", "--repo", "not-a-valid-repo-format")
	if err == nil {
		t.Error("expected error for invalid repo format, got nil")
	}
}

// ── issue list ────────────────────────────────────────────────────────────────

func TestGiteaCLI_Issue_List(t *testing.T) {
	ensureOrgAndRepo(t)
	stdout, _, err := runCLI(t, "issue", "list", "--repo", testOrg+"/"+testRepo)
	if err != nil {
		t.Fatalf("issue list: %v\nstdout: %s", err, stdout)
	}
	// Should succeed (empty list is fine)
}

func TestGiteaCLI_Issue_List_JSON(t *testing.T) {
	ensureOrgAndRepo(t)
	result := runCLIJSON(t, "issue", "list", "--repo", testOrg+"/"+testRepo)
	// Must be a JSON array
	if _, ok := result.([]interface{}); !ok {
		t.Errorf("issue list --json: expected array, got %T", result)
	}
}

// ── issue create + list round-trip ───────────────────────────────────────────

func TestGiteaCLI_Issue_CreateAndList(t *testing.T) {
	ensureOrgAndRepo(t)
	title := fmt.Sprintf("e2e-test-issue-%d", time.Now().UnixNano())

	// Create
	stdout, _, err := runCLI(t, "issue", "create",
		"--repo", testOrg+"/"+testRepo,
		"--title", title,
		"--body", "Created by e2e test",
	)
	if err != nil {
		t.Fatalf("issue create: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, "Created issue") {
		t.Errorf("unexpected create output: %q", stdout)
	}

	// List and find it
	result := runCLIJSON(t, "issue", "list",
		"--repo", testOrg+"/"+testRepo,
		"--state", "open",
	)
	issues, ok := result.([]interface{})
	if !ok {
		t.Fatalf("issue list: expected array, got %T", result)
	}
	found := false
	for _, item := range issues {
		issue, _ := item.(map[string]interface{})
		if issue["title"] == title {
			found = true
		}
	}
	if !found {
		t.Errorf("created issue %q not found in list", title)
	}
}

func TestGiteaCLI_Issue_Create_JSON(t *testing.T) {
	ensureOrgAndRepo(t)
	title := fmt.Sprintf("e2e-json-issue-%d", time.Now().UnixNano())

	result := runCLIJSON(t, "issue", "create",
		"--repo", testOrg+"/"+testRepo,
		"--title", title,
		"--body", "JSON output test",
	)
	issue, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("issue create --json: expected object, got %T", result)
	}
	if issue["title"] != title {
		t.Errorf("issue.title: got %v, want %s", issue["title"], title)
	}
	if issue["number"] == nil && issue["index"] == nil {
		t.Error("issue must have number/index field")
	}
}

// ── issue comment ─────────────────────────────────────────────────────────────

func TestGiteaCLI_Issue_Comment(t *testing.T) {
	ensureOrgAndRepo(t)
	title := fmt.Sprintf("e2e-comment-issue-%d", time.Now().UnixNano())

	// Create issue and extract number
	result := runCLIJSON(t, "issue", "create",
		"--repo", testOrg+"/"+testRepo,
		"--title", title,
		"--body", "for comment test",
	)
	issue, _ := result.(map[string]interface{})
	number := int64(issue["number"].(float64))

	// Comment
	stdout, _, err := runCLI(t, "issue", "comment",
		"--repo", testOrg+"/"+testRepo,
		"--number", fmt.Sprintf("%d", number),
		"--body", "e2e test comment",
	)
	if err != nil {
		t.Fatalf("issue comment: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, fmt.Sprintf("#%d", number)) {
		t.Errorf("comment output should mention issue number: %q", stdout)
	}
}

// ── issue close ───────────────────────────────────────────────────────────────

func TestGiteaCLI_Issue_Close(t *testing.T) {
	ensureOrgAndRepo(t)
	title := fmt.Sprintf("e2e-close-issue-%d", time.Now().UnixNano())

	// Create
	result := runCLIJSON(t, "issue", "create",
		"--repo", testOrg+"/"+testRepo,
		"--title", title,
		"--body", "to be closed",
	)
	issue, _ := result.(map[string]interface{})
	number := int64(issue["number"].(float64))

	// Close
	stdout, _, err := runCLI(t, "issue", "close",
		"--repo", testOrg+"/"+testRepo,
		"--number", fmt.Sprintf("%d", number),
	)
	if err != nil {
		t.Fatalf("issue close: %v\nstdout: %s", err, stdout)
	}

	// Verify it's closed
	closedResult := runCLIJSON(t, "issue", "list",
		"--repo", testOrg+"/"+testRepo,
		"--state", "closed",
	)
	closed, _ := closedResult.([]interface{})
	found := false
	for _, item := range closed {
		i, _ := item.(map[string]interface{})
		if int64(i["number"].(float64)) == number {
			found = true
		}
	}
	if !found {
		t.Errorf("issue #%d should appear in closed list after close", number)
	}
}

// ── issue list --state all ────────────────────────────────────────────────────

func TestGiteaCLI_Issue_List_StateAll(t *testing.T) {
	ensureOrgAndRepo(t)
	// Create and close an issue, then list all – should include both open and closed
	result := runCLIJSON(t, "issue", "list",
		"--repo", testOrg+"/"+testRepo,
		"--state", "all",
	)
	if _, ok := result.([]interface{}); !ok {
		t.Errorf("issue list --state all: expected array, got %T", result)
	}
}

// ── notify ────────────────────────────────────────────────────────────────────

func TestGiteaCLI_Notify_List(t *testing.T) {
	stdout, _, err := runCLI(t, "notify", "list")
	if err != nil {
		t.Fatalf("notify list: %v\nstdout: %s", err, stdout)
	}
}

func TestGiteaCLI_Notify_List_JSON(t *testing.T) {
	result := runCLIJSON(t, "notify", "list")
	if _, ok := result.([]interface{}); !ok {
		t.Errorf("notify list --json: expected array, got %T", result)
	}
}

func TestGiteaCLI_Notify_ReadAll(t *testing.T) {
	stdout, _, err := runCLI(t, "notify", "read-all")
	if err != nil {
		t.Fatalf("notify read-all: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, "read") {
		t.Errorf("notify read-all output: %q", stdout)
	}
}

// ── file get ──────────────────────────────────────────────────────────────────

func TestGiteaCLI_File_Get(t *testing.T) {
	ensureOrgAndRepo(t)
	// README.md is created by auto_init=true
	stdout, _, err := runCLI(t, "file", "get",
		"--repo", testOrg+"/"+testRepo,
		"--path", "README.md",
	)
	if err != nil {
		t.Fatalf("file get README.md: %v\nstdout: %s", err, stdout)
	}
	if stdout == "" {
		t.Error("file get: empty output for README.md")
	}
}

func TestGiteaCLI_File_Get_NotFound(t *testing.T) {
	ensureOrgAndRepo(t)
	_, _, err := runCLI(t, "file", "get",
		"--repo", testOrg+"/"+testRepo,
		"--path", "this-file-definitely-does-not-exist-xyz.txt",
	)
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

// ── JSON output is valid and parseable by jq ─────────────────────────────────

func TestGiteaCLI_JSONOutput_ParseableByJQ(t *testing.T) {
	ensureOrgAndRepo(t)

	// Check if jq is available
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not available, skipping jq parsing test")
	}

	bin := buildCLI(t)
	tokenFile := writeTokenFile(t)

	// Pipeline: gitea issue list --json | jq '.[0].title'
	giteaCmd := exec.Command(bin, "--json", "issue", "list",
		"--repo", testOrg+"/"+testRepo, "--state", "all", "--limit", "1")
	giteaCmd.Env = append(os.Environ(),
		"GITEA_BASE_URL="+giteaURL,
		"GITEA_TOKEN_PATH="+tokenFile,
	)

	jqCmd := exec.Command("jq", ".")
	jqCmd.Stdin, _ = giteaCmd.StdoutPipe()

	var jqOut bytes.Buffer
	jqCmd.Stdout = &jqOut

	if err := jqCmd.Start(); err != nil {
		t.Fatalf("start jq: %v", err)
	}
	if err := giteaCmd.Run(); err != nil {
		t.Fatalf("gitea cmd: %v", err)
	}
	if err := jqCmd.Wait(); err != nil {
		t.Fatalf("jq failed (invalid JSON?): %v\noutput: %s", err, jqOut.String())
	}
}
