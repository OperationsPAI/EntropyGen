package pgclient

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Client wraps a pgxpool.Pool and provides user storage operations.
type Client struct {
	pool *pgxpool.Pool
}

// User represents a platform user.
type User struct {
	ID           int
	Username     string
	PasswordHash string
	Role         string // "guest" | "member" | "admin"
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateUserInput holds fields for user creation.
type CreateUserInput struct {
	Username     string
	PasswordHash string
	Role         string
}

// UpdateUserInput holds mutable user fields.
type UpdateUserInput struct {
	Role         *string
	PasswordHash *string
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id            SERIAL PRIMARY KEY,
  username      VARCHAR(64) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  role          VARCHAR(32) NOT NULL DEFAULT 'member',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

// New connects to PostgreSQL, runs schema migration, and returns a Client.
func New(connURL string) (*Client, error) {
	pool, err := pgxpool.New(context.Background(), connURL)
	if err != nil {
		return nil, fmt.Errorf("pgclient: connect: %w", err)
	}
	c := &Client{pool: pool}
	if err := c.migrate(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgclient: migrate: %w", err)
	}
	return c, nil
}

// Close closes the underlying connection pool.
func (c *Client) Close() {
	c.pool.Close()
}

func (c *Client) migrate(ctx context.Context) error {
	_, err := c.pool.Exec(ctx, schema)
	return err
}

// CountUsers returns the total number of users.
func (c *Client) CountUsers(ctx context.Context) int {
	var count int
	_ = c.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count
}

// FindByUsername fetches a single user by username. Returns nil if not found.
func (c *Client) FindByUsername(ctx context.Context, username string) (*User, error) {
	row := c.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, role, created_at, updated_at
		 FROM users WHERE username = $1`, username)
	u := &User{}
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, nil //nolint:nilerr // not found
	}
	return u, nil
}

// ListUsers returns all users ordered by id.
func (c *Client) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := c.pool.Query(ctx,
		`SELECT id, username, password_hash, role, created_at, updated_at
		 FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("pgclient: list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("pgclient: scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CreateUser inserts a new user.
func (c *Client) CreateUser(ctx context.Context, in CreateUserInput) (*User, error) {
	u := &User{}
	err := c.pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, role)
		 VALUES ($1, $2, $3)
		 RETURNING id, username, password_hash, role, created_at, updated_at`,
		in.Username, in.PasswordHash, in.Role,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("pgclient: create user: %w", err)
	}
	return u, nil
}

// UpdateUser updates role and/or password for a user identified by username.
func (c *Client) UpdateUser(ctx context.Context, username string, in UpdateUserInput) (*User, error) {
	u, err := c.FindByUsername(ctx, username)
	if err != nil || u == nil {
		return nil, fmt.Errorf("pgclient: user not found: %s", username)
	}

	if in.Role != nil {
		u.Role = *in.Role
	}
	if in.PasswordHash != nil {
		u.PasswordHash = *in.PasswordHash
	}

	err = c.pool.QueryRow(ctx,
		`UPDATE users SET role=$1, password_hash=$2, updated_at=now()
		 WHERE username=$3
		 RETURNING id, username, password_hash, role, created_at, updated_at`,
		u.Role, u.PasswordHash, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("pgclient: update user: %w", err)
	}
	return u, nil
}

// DeleteUser removes a user by username.
func (c *Client) DeleteUser(ctx context.Context, username string) error {
	tag, err := c.pool.Exec(ctx, `DELETE FROM users WHERE username=$1`, username)
	if err != nil {
		return fmt.Errorf("pgclient: delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("pgclient: user not found: %s", username)
	}
	return nil
}

// HashPassword generates a bcrypt hash for the given plain-text password.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
