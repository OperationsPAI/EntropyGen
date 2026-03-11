package giteainit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	sdk "code.gitea.io/sdk/gitea"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config holds configuration for the init job.
type Config struct {
	GiteaURL             string // e.g. "http://gitea.gitea.svc:3000"
	AdminToken           string
	WebhookSecret        string
	OrgName              string // default: "platform"
	RepoName             string // default: "platform-demo"
	EventCollectorURL    string // e.g. "http://event-collector.control-plane.svc/webhooks/gitea"
	RunnerTokenNamespace string // K8s namespace for the runner token secret; default: "gitea"
}

// labelDef defines a label to be created.
type labelDef struct {
	Name        string
	Color       string
	Description string
}

// standardLabels are the 13 labels to create on the repository.
var standardLabels = []labelDef{
	{"priority/critical", "#FF0000", "Critical, needs immediate attention"},
	{"priority/high", "#FF6600", "High priority"},
	{"priority/medium", "#FFCC00", "Medium priority"},
	{"priority/low", "#00CC00", "Low priority"},
	{"type/bug", "#EE0701", "Bug report"},
	{"type/feature", "#84B6EB", "New feature request"},
	{"type/docs", "#0075CA", "Documentation"},
	{"type/refactor", "#E4E669", "Code refactoring"},
	{"type/test", "#F9D0C4", "Test related"},
	{"role/developer", "#7057FF", "For developer agents"},
	{"role/reviewer", "#008672", "For reviewer agents"},
	{"role/qa", "#E4E669", "For QA agents"},
	{"role/sre", "#0052CC", "For SRE agents"},
}

// webhookEvents are the events the webhook subscribes to.
var webhookEvents = []string{
	"push",
	"issues",
	"issue_comment",
	"pull_request",
	"pull_request_comment",
	"workflow_run",
}

// Runner executes initialization steps.
type Runner struct {
	client *sdk.Client
	cfg    Config
	log    *zap.Logger
}

// NewRunner creates a new Runner with the given configuration.
func NewRunner(cfg Config, log *zap.Logger) (*Runner, error) {
	client, err := sdk.NewClient(cfg.GiteaURL, sdk.SetToken(cfg.AdminToken))
	if err != nil {
		return nil, fmt.Errorf("create gitea client: %w", err)
	}
	return &Runner{
		client: client,
		cfg:    cfg,
		log:    log,
	}, nil
}

// Run executes all 7 initialization steps sequentially.
func (r *Runner) Run(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"wait for gitea readiness", r.waitForReady},
		{"create organization", r.createOrganization},
		{"create repository", r.createRepository},
		{"create standard labels", r.createLabels},
		{"create webhook", r.createWebhook},
		{"configure branch protection", r.configureBranchProtection},
		{"get runner registration token", r.getRunnerRegistrationToken},
	}

	for i, step := range steps {
		r.log.Info("executing step", zap.Int("step", i+1), zap.String("name", step.name))
		if err := step.fn(ctx); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.name, err)
		}
		r.log.Info("step completed", zap.Int("step", i+1), zap.String("name", step.name))
	}
	return nil
}

// waitForReady polls Gitea's version endpoint with exponential backoff until ready.
func (r *Runner) waitForReady(ctx context.Context) error {
	const totalTimeout = 5 * time.Minute
	const maxBackoff = 30 * time.Second

	deadline := time.Now().Add(totalTimeout)
	backoff := 1 * time.Second
	attempt := 0

	for {
		attempt++
		r.log.Info("checking gitea readiness", zap.Int("attempt", attempt), zap.Duration("backoff", backoff))

		ver, _, err := r.client.ServerVersion()
		if err == nil {
			r.log.Info("gitea is ready", zap.String("version", ver))
			return nil
		}

		r.log.Warn("gitea not ready yet", zap.Error(err))

		if time.Now().After(deadline) {
			return fmt.Errorf("gitea not ready after %v (%d attempts): %w", totalTimeout, attempt, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// createOrganization creates the platform organization if it does not exist.
func (r *Runner) createOrganization(ctx context.Context) error {
	_ = ctx
	_, resp, err := r.client.GetOrg(r.cfg.OrgName)
	if err == nil {
		r.log.Info("organization already exists, skipping", zap.String("org", r.cfg.OrgName))
		return nil
	}
	if resp != nil && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check organization %q: %w", r.cfg.OrgName, err)
	}

	_, _, err = r.client.CreateOrg(sdk.CreateOrgOption{
		Name:       r.cfg.OrgName,
		Visibility: sdk.VisibleTypePublic,
	})
	if err != nil {
		if isAlreadyExists(err) {
			r.log.Info("organization already exists (conflict), skipping", zap.String("org", r.cfg.OrgName))
			return nil
		}
		return fmt.Errorf("create organization %q: %w", r.cfg.OrgName, err)
	}

	r.log.Info("organization created", zap.String("org", r.cfg.OrgName))
	return nil
}

// createRepository creates the demo repository under the organization if it does not exist.
func (r *Runner) createRepository(ctx context.Context) error {
	_ = ctx
	_, resp, err := r.client.GetRepo(r.cfg.OrgName, r.cfg.RepoName)
	if err == nil {
		r.log.Info("repository already exists, skipping",
			zap.String("org", r.cfg.OrgName), zap.String("repo", r.cfg.RepoName))
		return nil
	}
	if resp != nil && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check repository %s/%s: %w", r.cfg.OrgName, r.cfg.RepoName, err)
	}

	_, _, err = r.client.CreateOrgRepo(r.cfg.OrgName, sdk.CreateRepoOption{
		Name:          r.cfg.RepoName,
		AutoInit:      true,
		DefaultBranch: "main",
		Private:       false,
	})
	if err != nil {
		if isAlreadyExists(err) {
			r.log.Info("repository already exists (conflict), skipping",
				zap.String("org", r.cfg.OrgName), zap.String("repo", r.cfg.RepoName))
			return nil
		}
		return fmt.Errorf("create repository %s/%s: %w", r.cfg.OrgName, r.cfg.RepoName, err)
	}

	r.log.Info("repository created",
		zap.String("org", r.cfg.OrgName), zap.String("repo", r.cfg.RepoName))
	return nil
}

// createLabels creates the standard labels on the repository, skipping any that already exist.
func (r *Runner) createLabels(ctx context.Context) error {
	_ = ctx
	existing, _, err := r.client.ListRepoLabels(r.cfg.OrgName, r.cfg.RepoName, sdk.ListLabelsOptions{
		ListOptions: sdk.ListOptions{Page: 1, PageSize: 50},
	})
	if err != nil {
		return fmt.Errorf("list labels: %w", err)
	}

	existingNames := make(map[string]bool, len(existing))
	for _, l := range existing {
		existingNames[l.Name] = true
	}

	created := 0
	skipped := 0
	for _, lbl := range standardLabels {
		if existingNames[lbl.Name] {
			r.log.Debug("label already exists, skipping", zap.String("label", lbl.Name))
			skipped++
			continue
		}

		_, _, err := r.client.CreateLabel(r.cfg.OrgName, r.cfg.RepoName, sdk.CreateLabelOption{
			Name:        lbl.Name,
			Color:       lbl.Color,
			Description: lbl.Description,
		})
		if err != nil {
			return fmt.Errorf("create label %q: %w", lbl.Name, err)
		}
		created++
	}

	r.log.Info("labels sync complete", zap.Int("created", created), zap.Int("skipped", skipped))
	return nil
}

// createWebhook creates a webhook on the repository if one pointing to the event collector does not exist.
func (r *Runner) createWebhook(ctx context.Context) error {
	_ = ctx
	hooks, _, err := r.client.ListRepoHooks(r.cfg.OrgName, r.cfg.RepoName, sdk.ListHooksOptions{
		ListOptions: sdk.ListOptions{Page: 1, PageSize: 50},
	})
	if err != nil {
		return fmt.Errorf("list hooks: %w", err)
	}

	for _, h := range hooks {
		if h.Config["url"] == r.cfg.EventCollectorURL {
			r.log.Info("webhook already exists, skipping", zap.String("url", r.cfg.EventCollectorURL))
			return nil
		}
	}

	_, _, err = r.client.CreateRepoHook(r.cfg.OrgName, r.cfg.RepoName, sdk.CreateHookOption{
		Type: sdk.HookTypeGitea,
		Config: map[string]string{
			"url":          r.cfg.EventCollectorURL,
			"secret":       r.cfg.WebhookSecret,
			"content_type": "json",
		},
		Events: webhookEvents,
		Active: true,
	})
	if err != nil {
		return fmt.Errorf("create webhook: %w", err)
	}

	r.log.Info("webhook created", zap.String("url", r.cfg.EventCollectorURL))
	return nil
}

// configureBranchProtection creates branch protection on main if it does not exist.
func (r *Runner) configureBranchProtection(ctx context.Context) error {
	_ = ctx
	protections, _, err := r.client.ListBranchProtections(r.cfg.OrgName, r.cfg.RepoName, sdk.ListBranchProtectionsOptions{})
	if err != nil {
		return fmt.Errorf("list branch protections: %w", err)
	}

	for _, p := range protections {
		if p.BranchName == "main" || p.RuleName == "main" {
			r.log.Info("branch protection for main already exists, skipping")
			return nil
		}
	}

	_, _, err = r.client.CreateBranchProtection(r.cfg.OrgName, r.cfg.RepoName, sdk.CreateBranchProtectionOption{
		BranchName:        "main",
		RuleName:          "main",
		EnablePush:        false,
		EnableStatusCheck: true,
		RequiredApprovals: 1,
	})
	if err != nil {
		return fmt.Errorf("create branch protection: %w", err)
	}

	r.log.Info("branch protection configured for main")
	return nil
}

// registrationTokenResponse is the JSON response from the runner registration endpoint.
type registrationTokenResponse struct {
	Token string `json:"token"`
}

// getRunnerRegistrationToken fetches a runner registration token via the admin API,
// writes it to a Kubernetes Secret, and also to a local file for dev/debugging.
func (r *Runner) getRunnerRegistrationToken(ctx context.Context) error {
	url := strings.TrimRight(r.cfg.GiteaURL, "/") + "/api/v1/admin/runners/registration-token"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+r.cfg.AdminToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request runner registration token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp registrationTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("parse registration token response: %w", err)
	}

	if tokenResp.Token == "" {
		return fmt.Errorf("empty registration token in response")
	}

	// Write to local file for dev/debugging purposes.
	const tokenPath = "/tmp/runner-registration-token"
	if err := os.WriteFile(tokenPath, []byte(tokenResp.Token), 0o600); err != nil {
		r.log.Warn("failed to write token to local file (non-fatal)", zap.String("path", tokenPath), zap.Error(err))
	} else {
		r.log.Info("runner registration token written to local file", zap.String("path", tokenPath))
	}

	// Create or update the Kubernetes Secret with the runner registration token.
	if err := r.writeRunnerTokenSecret(ctx, tokenResp.Token); err != nil {
		return fmt.Errorf("write runner token k8s secret: %w", err)
	}

	return nil
}

// writeRunnerTokenSecret creates or updates a Kubernetes Secret containing the runner registration token.
func (r *Runner) writeRunnerTokenSecret(ctx context.Context, token string) error {
	k8sCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("get in-cluster k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		return fmt.Errorf("create k8s clientset: %w", err)
	}

	namespace := r.cfg.RunnerTokenNamespace
	if namespace == "" {
		namespace = "gitea"
	}

	const secretName = "gitea-runner-token"
	secretsClient := clientset.CoreV1().Secrets(namespace)

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}

	existing, err := secretsClient.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("get existing secret %s/%s: %w", namespace, secretName, err)
		}
		// Secret does not exist, create it.
		if _, err := secretsClient.Create(ctx, desired, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create secret %s/%s: %w", namespace, secretName, err)
		}
		r.log.Info("k8s secret created with runner registration token",
			zap.String("namespace", namespace), zap.String("secret", secretName))
		return nil
	}

	// Secret exists, update it.
	existing.Data = desired.Data
	if _, err := secretsClient.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update secret %s/%s: %w", namespace, secretName, err)
	}
	r.log.Info("k8s secret updated with runner registration token",
		zap.String("namespace", namespace), zap.String("secret", secretName))
	return nil
}

// isAlreadyExists checks if an error indicates an "already exists" conflict.
func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "409")
}
