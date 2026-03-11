package gatewayctx

// Key type for context values to avoid collisions.
type Key string

const (
	AgentID   Key = "agent_id"
	AgentRole Key = "agent_role"
)
