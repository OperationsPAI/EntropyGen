package giteaclient

import (
	"context"
	"fmt"

	sdk "code.gitea.io/sdk/gitea"
)

// Client wraps the Gitea SDK client with simplified admin operations.
type Client struct {
	inner *sdk.Client
}

// New creates a new Gitea admin client.
// baseURL example: "http://localhost:3000"
// adminToken: Gitea admin API token
func New(baseURL, adminToken string) (*Client, error) {
	inner, err := sdk.NewClient(baseURL, sdk.SetToken(adminToken))
	if err != nil {
		return nil, fmt.Errorf("gitea client init: %w", err)
	}
	return &Client{inner: inner}, nil
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
func (c *Client) CreateToken(_ context.Context, username, tokenName string) (string, error) {
	token, _, err := c.inner.CreateAccessToken(sdk.CreateAccessTokenOption{
		Name: tokenName,
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

// DeleteUser deletes a Gitea user by username.
func (c *Client) DeleteUser(_ context.Context, username string) error {
	_, err := c.inner.AdminDeleteUser(username)
	if err != nil {
		return fmt.Errorf("delete user %q: %w", username, err)
	}
	return nil
}

// Version returns the Gitea server version (useful for health checks).
func (c *Client) Version(_ context.Context) (string, error) {
	ver, _, err := c.inner.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("gitea version: %w", err)
	}
	return ver, nil
}
