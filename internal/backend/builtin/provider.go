package builtin

import (
	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
)

// Provider implements k8sclient.BuiltinContentProvider using embedded files.
type Provider struct{}

var _ k8sclient.BuiltinContentProvider = (*Provider)(nil)

func (p *Provider) ReadSOUL() string                          { return ReadSOUL() }
func (p *Provider) ReadPrompt() string                        { return ReadPrompt() }
func (p *Provider) ReadPromptForRole(role string) string      { return ReadPromptForRole(role) }
func (p *Provider) BuildAgentsMD(role string) string          { return BuildAgentsMD(role) }
func (p *Provider) BuiltinSkillsForRole(role string) []string { return BuiltinSkillsForRole(role) }
func (p *Provider) ReadSkill(name string) string              { return ReadSkill(name) }
