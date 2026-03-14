package models

import (
	"encoding/json"
	"time"
)

// EventType constants for all event types in the system
const (
	EventTypeGatewayLLMInference = "gateway.llm_inference"
	EventTypeGatewayGiteaAPI     = "gateway.gitea_api"
	EventTypeGatewayGitHTTP      = "gateway.git_http"
	EventTypeGatewayHeartbeat    = "gateway.heartbeat"
	EventTypeGiteaIssueOpen      = "gitea.issue_open"
	EventTypeGiteaIssueComment   = "gitea.issue_comment"
	EventTypeGiteaPush           = "gitea.push"
	EventTypeGiteaPROpen         = "gitea.pr_open"
	EventTypeGiteaPRReview       = "gitea.pr_review"
	EventTypeGiteaPRMerge        = "gitea.pr_merge"
	EventTypeGiteaCIStatus       = "gitea.ci_status"
	EventTypeIssueAssignedByAdmin = "issue.assigned_by_admin"
	EventTypeOperatorAgentAlert  = "operator.agent_alert"
	EventTypeK8SPodStatus        = "k8s.pod_status"
)

// Event is the unified event structure for all events in the system.
// All events flowing through Redis Streams use this format.
type Event struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp time.Time       `json:"timestamp"`
	AgentID   string          `json:"agent_id"`
	AgentRole string          `json:"agent_role"`
	Payload   json.RawMessage `json:"payload"`
}

// GatewayLLMPayload is the payload for gateway.llm_inference events
type GatewayLLMPayload struct {
	TraceID    string `json:"trace_id"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	Model      string `json:"model"`
	TokensIn   int    `json:"tokens_in"`
	TokensOut  int    `json:"tokens_out"`
	LatencyMs  int64  `json:"latency_ms"`
}

// GatewayGiteaPayload is the payload for gateway.gitea_api events
type GatewayGiteaPayload struct {
	TraceID    string `json:"trace_id"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
}

// GatewayHeartbeatPayload is the payload for gateway.heartbeat events
type GatewayHeartbeatPayload struct {
	TraceID string `json:"trace_id"`
	Status  string `json:"status"`
}

// GiteaIssuePayload is the payload for gitea.issue_open / gitea.issue_comment events
type GiteaIssuePayload struct {
	Repo        string   `json:"repo"`
	IssueNumber int      `json:"issue_number"`
	Title       string   `json:"title,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Creator     string   `json:"creator,omitempty"`
}

// GiteaPRMergePayload is the payload for gitea.pr_merge events
type GiteaPRMergePayload struct {
	Repo         string `json:"repo"`
	PRNumber     int    `json:"pr_number"`
	Title        string `json:"title"`
	MergedBy     string `json:"merged_by"`
	ClosesIssues []int  `json:"closes_issues,omitempty"`
}

// GiteaCIStatusPayload is the payload for gitea.ci_status events
type GiteaCIStatusPayload struct {
	Repo     string `json:"repo"`
	Workflow string `json:"workflow"`
	Status   string `json:"status"`
	Commit   string `json:"commit"`
	PRNumber int    `json:"pr_number,omitempty"`
}

// IssueAssignedByAdminPayload is the payload for issue.assigned_by_admin events
type IssueAssignedByAdminPayload struct {
	Repo        string   `json:"repo"`
	IssueNumber int64    `json:"issue_number"`
	IssueURL    string   `json:"issue_url"`
	Title       string   `json:"title"`
	Labels      []string `json:"labels,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Assignee    string   `json:"assignee"`
}

// K8SAgentAlertPayload is the payload for operator.agent_alert events
type K8SAgentAlertPayload struct {
	AlertType    string `json:"alert_type"`
	RestartCount int    `json:"restart_count,omitempty"`
	Message      string `json:"message"`
}

// K8SPodStatusPayload is the payload for k8s.pod_status events
type K8SPodStatusPayload struct {
	PodName        string `json:"pod_name"`
	Namespace      string `json:"namespace"`
	Status         string `json:"status"`
	PreviousStatus string `json:"previous_status,omitempty"`
}

// Alert type constants
const (
	AlertTypeCrashLoop         = "agent.crash_loop"
	AlertTypeOOMKilled         = "agent.oom_killed"
	AlertTypeHeartbeatTimeout  = "agent.heartbeat_timeout"
	AlertTypeTokenBudgetExceed = "agent.token_budget_exceeded"
	AlertTypeOOMExpanded       = "agent.oom_expanded"
)
