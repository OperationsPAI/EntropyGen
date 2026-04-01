package config

import (
	"fmt"
	"os"
)

// DefaultAgentRuntimeImage is the default container image for agent pods.
const DefaultAgentRuntimeImage = "registry.local/agent-runtime:latest"

// MustEnv returns the value of the environment variable or panics if not set.
func MustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}

// EnvOr returns the value of the environment variable or the default value.
func EnvOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// AgentRuntimeImage returns the configured agent runtime image.
func AgentRuntimeImage() string {
	return EnvOr("AGENT_RUNTIME_IMAGE", DefaultAgentRuntimeImage)
}
