// Package e2e contains end-to-end tests for gitea-init-job against the live Gitea instance.
//
// Prerequisites:
//   - Gitea running at http://localhost:3000  (admin: gitea-admin / gitea-admin123)
//
// Run:
//
//	go test ./tests/e2e/ -v -run TestGiteaInit
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	giteainit "github.com/entropyGen/entropyGen/internal/gitea-init/init"
)

// newInitConfig returns a Config wired to the live dev Gitea instance.
// Org/repo names are unique per test to allow parallel runs and clean teardown.
func newInitConfig(t *testing.T, suffix string) giteainit.Config {
	t.Helper()
	// Use the gitea-admin token created in dev setup
	return giteainit.Config{
		GiteaURL:          giteaURL,
		AdminToken:        giteaToken(t),
		WebhookSecret:     "test-webhook-secret",
		OrgName:           fmt.Sprintf("init-test-org-%s", suffix),
		RepoName:          "init-test-repo",
		EventCollectorURL: "http://localhost:9999/webhooks/gitea", // unused service, won't be called
	}
}

func newTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	return zaptest.NewLogger(t)
}

// cleanupOrg deletes the test org (best-effort, non-fatal).
func cleanupOrg(t *testing.T, org string) {
	t.Helper()
	client := &giteaAdminClient{t: t}
	// Delete each repo first, then the org
	client.do("DELETE", "/orgs/"+org+"/repos/init-test-repo", nil)
	client.do("DELETE", "/orgs/"+org, nil)
}

// ── Step 1: Wait for readiness ────────────────────────────────────────────────

func TestGiteaInit_Step1_WaitsForReadiness(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := newInitConfig(t, suffix)
	t.Cleanup(func() { cleanupOrg(t, cfg.OrgName) })

	runner, err := giteainit.NewRunner(cfg, newTestLogger(t))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// waitForReady is called implicitly by Run; verify Run succeeds
	// (Gitea is already up, so this should complete quickly)
	if err := runner.Run(ctx); err != nil {
		// Step 7 (runner registration token via admin API) may fail without K8s.
		// Accept errors only from that specific step.
		if !strings.Contains(err.Error(), "step 7") &&
			!strings.Contains(err.Error(), "runner") &&
			!strings.Contains(err.Error(), "in-cluster") {
			t.Errorf("Run failed unexpectedly: %v", err)
		}
	}
}

// ── Step 2+3: Create org and repo ────────────────────────────────────────────

func TestGiteaInit_Step2_CreateOrganization(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := newInitConfig(t, suffix)
	t.Cleanup(func() { cleanupOrg(t, cfg.OrgName) })

	runner, err := giteainit.NewRunner(cfg, newTestLogger(t))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	ctx := context.Background()
	// Run up to step 2 (the first error in step 7 is acceptable)
	runIgnoringStep7Error(t, runner, ctx)

	// Verify org exists
	resp, _ := httpClient.Get(giteaURL + "/api/v1/orgs/" + cfg.OrgName)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("org %s: expected 200 after init, got %d", cfg.OrgName, resp.StatusCode)
	}
}

func TestGiteaInit_Step3_CreateRepository(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := newInitConfig(t, suffix)
	t.Cleanup(func() { cleanupOrg(t, cfg.OrgName) })

	runner, _ := giteainit.NewRunner(cfg, newTestLogger(t))
	runIgnoringStep7Error(t, runner, context.Background())

	// Verify repo exists
	resp, _ := httpClient.Get(giteaURL + "/api/v1/repos/" + cfg.OrgName + "/" + cfg.RepoName)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("repo %s/%s: expected 200 after init, got %d",
			cfg.OrgName, cfg.RepoName, resp.StatusCode)
	}
}

// ── Step 4: Standard labels ───────────────────────────────────────────────────

func TestGiteaInit_Step4_Creates13Labels(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := newInitConfig(t, suffix)
	t.Cleanup(func() { cleanupOrg(t, cfg.OrgName) })

	runner, _ := giteainit.NewRunner(cfg, newTestLogger(t))
	runIgnoringStep7Error(t, runner, context.Background())

	// Fetch labels via Gitea API
	req, _ := http.NewRequest("GET",
		giteaURL+"/api/v1/repos/"+cfg.OrgName+"/"+cfg.RepoName+"/labels?limit=50", nil)
	req.Header.Set("Authorization", "token "+giteaToken(t))
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list labels: got %d", resp.StatusCode)
	}

	var labels []map[string]interface{}
	decodeJSON(t, resp.Body, &labels)

	if len(labels) != 13 {
		names := make([]string, len(labels))
		for i, l := range labels {
			names[i], _ = l["name"].(string)
		}
		t.Errorf("expected 13 labels, got %d: %v", len(labels), names)
	}

	// Verify all expected label names are present
	expected := []string{
		"priority/critical", "priority/high", "priority/medium", "priority/low",
		"type/bug", "type/feature", "type/docs", "type/refactor", "type/test",
		"role/developer", "role/reviewer", "role/qa", "role/sre",
	}
	labelMap := make(map[string]bool, len(labels))
	for _, l := range labels {
		name, _ := l["name"].(string)
		labelMap[name] = true
	}
	for _, name := range expected {
		if !labelMap[name] {
			t.Errorf("missing label: %q", name)
		}
	}
}

// ── Step 5: Webhook ───────────────────────────────────────────────────────────

func TestGiteaInit_Step5_CreatesWebhook(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := newInitConfig(t, suffix)
	t.Cleanup(func() { cleanupOrg(t, cfg.OrgName) })

	runner, _ := giteainit.NewRunner(cfg, newTestLogger(t))
	runIgnoringStep7Error(t, runner, context.Background())

	req, _ := http.NewRequest("GET",
		giteaURL+"/api/v1/repos/"+cfg.OrgName+"/"+cfg.RepoName+"/hooks", nil)
	req.Header.Set("Authorization", "token "+giteaToken(t))
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("list hooks: %v", err)
	}
	defer resp.Body.Close()

	var hooks []map[string]interface{}
	decodeJSON(t, resp.Body, &hooks)

	found := false
	for _, h := range hooks {
		config, _ := h["config"].(map[string]interface{})
		if url, _ := config["url"].(string); url == cfg.EventCollectorURL {
			found = true
			events, _ := h["events"].([]interface{})
			eventSet := make(map[string]bool)
			for _, e := range events {
				if s, ok := e.(string); ok {
					eventSet[s] = true
				}
			}
			required := []string{"push", "issues", "issue_comment", "pull_request", "pull_request_comment", "workflow_run"}
			for _, req := range required {
				if !eventSet[req] {
					t.Errorf("webhook missing event %q (has: %v)", req, eventSet)
				}
			}
		}
	}
	if !found {
		t.Errorf("webhook pointing to %q not found", cfg.EventCollectorURL)
	}
}

// ── Step 6: Branch protection ─────────────────────────────────────────────────

func TestGiteaInit_Step6_BranchProtection(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := newInitConfig(t, suffix)
	t.Cleanup(func() { cleanupOrg(t, cfg.OrgName) })

	runner, _ := giteainit.NewRunner(cfg, newTestLogger(t))
	runIgnoringStep7Error(t, runner, context.Background())

	req, _ := http.NewRequest("GET",
		giteaURL+"/api/v1/repos/"+cfg.OrgName+"/"+cfg.RepoName+"/branch_protections", nil)
	req.Header.Set("Authorization", "token "+giteaToken(t))
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("list branch protections: %v", err)
	}
	defer resp.Body.Close()

	var protections []map[string]interface{}
	decodeJSON(t, resp.Body, &protections)

	found := false
	for _, p := range protections {
		branch, _ := p["branch_name"].(string)
		ruleName, _ := p["rule_name"].(string)
		if branch == "main" || ruleName == "main" {
			found = true
			// Verify required approvals
			if approvals, _ := p["required_approvals"].(float64); approvals < 1 {
				t.Errorf("branch protection: required_approvals=%v, want >=1", approvals)
			}
		}
	}
	if !found {
		t.Errorf("branch protection for 'main' not found after init")
	}
}

// ── Idempotency: running init twice should not fail ───────────────────────────

func TestGiteaInit_Idempotency(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := newInitConfig(t, suffix)
	t.Cleanup(func() { cleanupOrg(t, cfg.OrgName) })

	logger := newTestLogger(t)

	// First run
	runner1, err := giteainit.NewRunner(cfg, logger)
	if err != nil {
		t.Fatalf("NewRunner (1st): %v", err)
	}
	runIgnoringStep7Error(t, runner1, context.Background())

	// Second run – must not fail on already-created resources
	runner2, err := giteainit.NewRunner(cfg, logger)
	if err != nil {
		t.Fatalf("NewRunner (2nd): %v", err)
	}
	runIgnoringStep7Error(t, runner2, context.Background())

	// Verify there's still exactly 13 labels (no duplicates)
	req, _ := http.NewRequest("GET",
		giteaURL+"/api/v1/repos/"+cfg.OrgName+"/"+cfg.RepoName+"/labels?limit=50", nil)
	req.Header.Set("Authorization", "token "+giteaToken(t))
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	defer resp.Body.Close()

	var labels []map[string]interface{}
	decodeJSON(t, resp.Body, &labels)
	if len(labels) != 13 {
		t.Errorf("idempotency: expected 13 labels after 2nd run, got %d (duplicates?)", len(labels))
	}

	// Verify there's exactly 1 webhook (no duplicates)
	hookReq, _ := http.NewRequest("GET",
		giteaURL+"/api/v1/repos/"+cfg.OrgName+"/"+cfg.RepoName+"/hooks", nil)
	hookReq.Header.Set("Authorization", "token "+giteaToken(t))
	hookResp, err := httpClient.Do(hookReq)
	if err != nil {
		t.Fatalf("list hooks: %v", err)
	}
	defer hookResp.Body.Close()

	var hooks []map[string]interface{}
	decodeJSON(t, hookResp.Body, &hooks)

	count := 0
	for _, h := range hooks {
		config, _ := h["config"].(map[string]interface{})
		if url, _ := config["url"].(string); url == cfg.EventCollectorURL {
			count++
		}
	}
	if count != 1 {
		t.Errorf("idempotency: expected 1 webhook, got %d (duplicate?)", count)
	}
}

// ── Webhook event list matches design ─────────────────────────────────────────

func TestGiteaInit_WebhookEvents_MatchDesign(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := newInitConfig(t, suffix)
	t.Cleanup(func() { cleanupOrg(t, cfg.OrgName) })

	runner, _ := giteainit.NewRunner(cfg, newTestLogger(t))
	runIgnoringStep7Error(t, runner, context.Background())

	req, _ := http.NewRequest("GET",
		giteaURL+"/api/v1/repos/"+cfg.OrgName+"/"+cfg.RepoName+"/hooks", nil)
	req.Header.Set("Authorization", "token "+giteaToken(t))
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("list hooks: %v", err)
	}
	defer resp.Body.Close()

	var hooks []map[string]interface{}
	decodeJSON(t, resp.Body, &hooks)

	for _, h := range hooks {
		config, _ := h["config"].(map[string]interface{})
		if url, _ := config["url"].(string); url != cfg.EventCollectorURL {
			continue
		}
		events, _ := h["events"].([]interface{})
		eventSet := make(map[string]bool)
		for _, e := range events {
			if s, ok := e.(string); ok {
				eventSet[s] = true
			}
		}
		// These are the events specified in the design (all supported by Gitea 1.25+)
		supportedEvents := []string{
			"push",
			"issues",
			"issue_comment",
			"pull_request",
			"pull_request_comment",
			"workflow_run",
		}
		for _, ev := range supportedEvents {
			if !eventSet[ev] {
				t.Errorf("webhook missing required event %q", ev)
			}
		}
		// Must NOT have the old wrong events
		if eventSet["create"] {
			t.Error("webhook should not have 'create' event (removed in phase 7 fix)")
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// runIgnoringStep7Error runs the full init pipeline but tolerates step 7 failures
// (runner registration token requires K8s in-cluster config which is unavailable in dev).
func runIgnoringStep7Error(t *testing.T, runner *giteainit.Runner, ctx context.Context) {
	t.Helper()
	if err := runner.Run(ctx); err != nil {
		if isStep7Error(err) {
			t.Logf("step 7 (runner token K8s write) failed as expected without K8s: %v", err)
			return
		}
		t.Errorf("Run: %v", err)
	}
}

// isStep7Error returns true if the error originates from step 7 (runner registration token).
func isStep7Error(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "step 7") ||
		strings.Contains(s, "runner") ||
		strings.Contains(s, "in-cluster") ||
		strings.Contains(s, "k8s") ||
		strings.Contains(s, "kubernetes")
}

// decodeJSON decodes JSON from a reader into dst, failing the test on error.
func decodeJSON(t *testing.T, r io.Reader, dst any) {
	t.Helper()
	if err := json.NewDecoder(r).Decode(dst); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}
