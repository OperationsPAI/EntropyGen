package giteaclient

import (
	"context"
	"fmt"
	"net/http"

	sdk "code.gitea.io/sdk/gitea"
)

// Client wraps the Gitea SDK client with simplified admin operations.
type Client struct {
	inner   *sdk.Client
	baseURL string
	token   string
}

// New creates a new Gitea admin client.
// baseURL example: "http://localhost:3000"
// adminToken: Gitea admin API token
func New(baseURL, adminToken string) (*Client, error) {
	inner, err := sdk.NewClient(baseURL, sdk.SetToken(adminToken))
	if err != nil {
		return nil, fmt.Errorf("gitea client init: %w", err)
	}
	return &Client{inner: inner, baseURL: baseURL, token: adminToken}, nil
}

// CreateUser creates a new Gitea user. Must be called with admin credentials.
func (c *Client) CreateUser(_ context.Context, username, email, password string) error {
	mustChangePassword := false
	_, _, err := c.inner.AdminCreateUser(sdk.CreateUserOption{
		Username:           username,
		Email:              email,
		Password:           password,
		MustChangePassword: &mustChangePassword,
		SendNotify:         false,
	})
	if err != nil {
		return fmt.Errorf("create user %q: %w", username, err)
	}
	return nil
}

// CreateToken creates an API token for the given user. Returns the raw token value.
// Gitea requires BasicAuth (username+password) to create tokens, not admin token auth.
func (c *Client) CreateToken(_ context.Context, username, tokenName string) (string, error) {
	return "", fmt.Errorf("use CreateTokenWithPassword instead")
}

// CreateTokenWithPassword creates an API token by authenticating as the user with BasicAuth.
func (c *Client) CreateTokenWithPassword(ctx context.Context, username, password, tokenName string) (string, error) {
	userClient, err := sdk.NewClient(c.baseURL, sdk.SetBasicAuth(username, password))
	if err != nil {
		return "", fmt.Errorf("create user client for %q: %w", username, err)
	}

	token, _, err := userClient.CreateAccessToken(sdk.CreateAccessTokenOption{
		Name:   tokenName,
		Scopes: []sdk.AccessTokenScope{sdk.AccessTokenScopeAll},
	})
	if err != nil {
		return "", fmt.Errorf("create token for %q: %w", username, err)
	}
	return token.Token, nil
}

// AddCollaborator adds a user as a collaborator to a repository.
// permission: "read", "write", "admin"
func (c *Client) AddCollaborator(_ context.Context, owner, repo, collaborator, permission string) error {
	accessMode := sdk.AccessMode(permission)
	_, err := c.inner.AddCollaborator(owner, repo, collaborator, sdk.AddCollaboratorOption{
		Permission: &accessMode,
	})
	if err != nil {
		return fmt.Errorf("add collaborator %q to %s/%s: %w", collaborator, owner, repo, err)
	}
	return nil
}

// DeleteUser deletes a Gitea user by username with purge=true to force-remove
// even when the user owns repositories.
func (c *Client) DeleteUser(_ context.Context, username string) error {
	url := fmt.Sprintf("%s/api/v1/admin/users/%s?purge=true", c.baseURL, username)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build delete request for %q: %w", username, err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete user %q: %w", username, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil // already gone
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete user %q: HTTP %d", username, resp.StatusCode)
	}
	return nil
}

// IssueResult holds the data returned after creating an issue.
type IssueResult struct {
	Number  int64  // issue index (e.g. 17)
	HTMLURL string // full URL to the issue page
}

// CreateIssue creates a new issue in the given repository and assigns it to the
// specified user. Labels are resolved from names to IDs automatically.
// owner is the repository organization/user, repo is the repository name.
func (c *Client) CreateIssue(_ context.Context, owner, repo string, opts CreateIssueOpts) (*IssueResult, error) {
	createOpt := sdk.CreateIssueOption{
		Title:     opts.Title,
		Body:      opts.Body,
		Assignees: []string{opts.Assignee},
	}

	if len(opts.Labels) > 0 {
		labelIDs, err := c.resolveLabelIDs(owner, repo, opts.Labels)
		if err != nil {
			return nil, err
		}
		createOpt.Labels = labelIDs
	}

	issue, _, err := c.inner.CreateIssue(owner, repo, createOpt)
	if err != nil {
		return nil, fmt.Errorf("create issue in %s/%s: %w", owner, repo, err)
	}

	return &IssueResult{
		Number:  issue.Index,
		HTMLURL: issue.HTMLURL,
	}, nil
}

// CreateIssueOpts holds parameters for CreateIssue.
type CreateIssueOpts struct {
	Title    string
	Body     string
	Labels   []string
	Assignee string
}

// resolveLabelIDs maps label names to their IDs for the given repository.
func (c *Client) resolveLabelIDs(owner, repo string, names []string) ([]int64, error) {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	const pageSize = 50
	var allLabels []*sdk.Label
	page := 1
	for {
		labels, _, err := c.inner.ListRepoLabels(owner, repo, sdk.ListLabelsOptions{
			ListOptions: sdk.ListOptions{Page: page, PageSize: pageSize},
		})
		if err != nil {
			return nil, fmt.Errorf("list labels for %s/%s: %w", owner, repo, err)
		}
		allLabels = append(allLabels, labels...)
		if len(labels) < pageSize {
			break
		}
		page++
	}

	ids := make([]int64, 0, len(names))
	found := make(map[string]bool, len(names))
	for _, l := range allLabels {
		if nameSet[l.Name] {
			ids = append(ids, l.ID)
			found[l.Name] = true
		}
	}

	for _, name := range names {
		if !found[name] {
			return nil, fmt.Errorf("label %q not found in %s/%s", name, owner, repo)
		}
	}
	return ids, nil
}

// Version returns the Gitea server version (useful for health checks).
func (c *Client) Version(_ context.Context) (string, error) {
	ver, _, err := c.inner.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("gitea version: %w", err)
	}
	return ver, nil
}
