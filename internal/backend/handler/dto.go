package handler

import "encoding/json"

// CreateAgentRequest is the body for POST /agents.
type CreateAgentRequest struct {
	Name string          `json:"name" binding:"required"`
	Spec json.RawMessage `json:"spec" binding:"required"`
}

// AssignIssueRequest is the body for POST /agents/:name/assign-issue.
type AssignIssueRequest struct {
	Repo     string   `json:"repo" binding:"required" example:"ai-team/webapp"`
	Title    string   `json:"title" binding:"required"`
	Body     string   `json:"body" binding:"required"`
	Labels   []string `json:"labels"`
	Priority string   `json:"priority" example:"medium"`
}

// AssignIssueResponseData is the data for POST /agents/:name/assign-issue.
type AssignIssueResponseData struct {
	IssueNumber int    `json:"issue_number"`
	IssueURL    string `json:"issue_url"`
}

// LoginRequest is the body for POST /auth/login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponseData is the data for POST /auth/login.
type LoginResponseData struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UserInfo is returned by GET /auth/me.
type UserInfo struct {
	Username interface{} `json:"username"`
	Role     interface{} `json:"role"`
}

// CreateUserRequest is the body for POST /users.
type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required" example:"member"`
}

// UpdateUserRequest is the body for PUT /users/:username.
type UpdateUserRequest struct {
	Role     *string `json:"role,omitempty"`
	Password *string `json:"password,omitempty"`
}

// UpdateRoleRequest is the body for PATCH /roles/:name.
type UpdateRoleRequest struct {
	Description string `json:"description" binding:"required"`
}

// PutFileRequest is the body for PUT /roles/:name/files/*filepath.
type PutFileRequest struct {
	Content string `json:"content"`
}

// RenameFileRequest is the body for POST /roles/:name/rename-file.
type RenameFileRequest struct {
	OldName string `json:"old_name" binding:"required"`
	NewName string `json:"new_name" binding:"required"`
}

// RuntimeTypeEntry describes an available agent runtime type.
type RuntimeTypeEntry struct {
	Type    string `json:"type"`
	Image   string `json:"image"`
	Default bool   `json:"default"`
}

// HealthModelResponse is the response for POST /llm/health/:id.
type HealthModelResponse struct {
	Status    string `json:"status" example:"healthy"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ConfigResponse is the response for GET /config.
type ConfigResponse struct {
	GiteaBaseURL string `json:"gitea_base_url"`
}
