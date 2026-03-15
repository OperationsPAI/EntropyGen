// Package builtin provides embedded builtin role templates and skills.
//
// The builtin-role/ and skills/ directories are copies of agent-runtime/builtin-role/
// and agent-runtime/skills/. The Dockerfile (build/backend/Dockerfile) copies the
// canonical source into this directory at build time. For local development,
// the committed copies allow `go build` to work without extra steps.
package builtin

import (
	"embed"
	"strings"
)

//go:embed builtin-role/*
var builtinFS embed.FS

//go:embed skills/**/SKILL.md
var skillsFS embed.FS

// ReadSOUL returns the default SOUL.md template content.
func ReadSOUL() string {
	data, err := builtinFS.ReadFile("builtin-role/SOUL.md")
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadPrompt returns the default PROMPT.md template content.
func ReadPrompt() string {
	data, err := builtinFS.ReadFile("builtin-role/PROMPT.md")
	if err != nil {
		return ""
	}
	return string(data)
}

// BuildAgentsMD constructs the AGENTS.md content by combining
// the base template with the role-specific template.
func BuildAgentsMD(role string) string {
	base, err := builtinFS.ReadFile("builtin-role/agents-templates/base.md")
	if err != nil {
		return ""
	}

	roleFile := roleTemplateFile(role)
	if roleFile == "" {
		return string(base)
	}

	rolePart, err := builtinFS.ReadFile("builtin-role/agents-templates/" + roleFile)
	if err != nil {
		return string(base)
	}

	return string(base) + "\n" + string(rolePart)
}

// BuiltinSkillsForRole returns the list of builtin skill names for a given role.
func BuiltinSkillsForRole(role string) []string {
	skills := []string{"gitea-api"}
	switch role {
	case "developer":
		skills = append(skills, "git-ops")
	case "sre":
		skills = append(skills, "git-ops", "kubectl-ops")
	}
	return skills
}

// ReadSkill reads a skill file from the embedded skills filesystem.
func ReadSkill(name string) string {
	data, err := skillsFS.ReadFile("skills/" + name + "/SKILL.md")
	if err != nil {
		return "# " + name + " Skill\n(skill content not found)\n"
	}
	return string(data)
}

// SkillKey converts a skill path like "git-ops/SKILL.md" to ConfigMap key "git-ops__SKILL.md".
func SkillKey(path string) string {
	return strings.ReplaceAll(path, "/", "__")
}

func roleTemplateFile(role string) string {
	switch role {
	case "developer":
		return "developer.md"
	case "reviewer":
		return "reviewer.md"
	case "sre":
		return "sre.md"
	case "observer":
		return "observer.md"
	default:
		return ""
	}
}
