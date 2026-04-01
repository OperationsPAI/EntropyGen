package observer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AgentState represents the runtime state persisted by the agent process
// in <workspace>/state.json.
type AgentState struct {
	LastNotificationCheck string     `json:"last_notification_check,omitempty"`
	CurrentTask           *StateTask `json:"current_task,omitempty"`
}

// StateTask describes the issue or PR the agent is currently working on.
type StateTask struct {
	Type      string `json:"type"`
	ID        int    `json:"id"`
	Title     string `json:"title,omitempty"`
	Repo      string `json:"repo,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
}

// ReadState reads and parses the agent state.json file from the workspace directory.
// Returns an empty AgentState (not an error) when the file does not exist
// or contains invalid JSON, since this data is best-effort.
func ReadState(workspaceDir string) AgentState {
	var state AgentState
	data, err := os.ReadFile(filepath.Join(workspaceDir, "state.json"))
	if err != nil {
		return state
	}
	_ = json.Unmarshal(data, &state)
	return state
}
