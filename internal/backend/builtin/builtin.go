// Package builtin provides embedded builtin role templates and skills.
//
// The builtin-role/ and skills/ directories are copies of agent-runtime/builtin-role/
// and agent-runtime/skills/. The Dockerfile (build/backend/Dockerfile) copies the
// canonical source into this directory at build time. For local development,
// the committed copies allow `go build` to work without extra steps.
package builtin

import (
	"bytes"
	"embed"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/entropyGen/entropyGen/internal/backend/k8sclient"
)

//go:embed all:builtin-role
var builtinFS embed.FS

//go:embed skills/**/SKILL.md
var skillsFS embed.FS

// roleFrontmatter is the YAML structure at the top of role template files.
type roleFrontmatter struct {
	Label       string   `yaml:"label"`
	Description string   `yaml:"description"`
	Skills      []string `yaml:"skills"`
	Permissions []string `yaml:"permissions"`
}

// parseFrontmatter splits YAML frontmatter (delimited by "---") from the
// markdown body. Returns the YAML bytes and the remaining body. If no
// frontmatter is found, yamlBytes is nil and body is the full content.
func parseFrontmatter(content []byte) (yamlBytes []byte, body []byte) {
	trimmed := bytes.TrimLeft(content, "\n\r ")
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return nil, content
	}
	// Find end delimiter
	rest := trimmed[3:]
	rest = bytes.TrimLeft(rest, " ")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return nil, content
	}
	yamlBytes = rest[:end]
	body = rest[end+4:] // skip "\n---"
	// Trim leading newline from body
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}
	return yamlBytes, body
}

// parseRoleFrontmatter parses the frontmatter of a role template file.
func parseRoleFrontmatter(content []byte) (*roleFrontmatter, error) {
	yamlBytes, _ := parseFrontmatter(content)
	if yamlBytes == nil {
		return nil, nil
	}
	var fm roleFrontmatter
	if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
		return nil, err
	}
	return &fm, nil
}

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
// the base template with the role-specific template (frontmatter stripped).
func BuildAgentsMD(role string) string {
	base, err := builtinFS.ReadFile("builtin-role/agents-templates/base.md")
	if err != nil {
		return ""
	}

	fileName := roleTemplateFile(role)
	if fileName == "" {
		return string(base)
	}

	rolePart, err := builtinFS.ReadFile("builtin-role/agents-templates/" + fileName)
	if err != nil {
		return string(base)
	}

	// Strip frontmatter from role template before concatenation
	_, body := parseFrontmatter(rolePart)
	return string(base) + "\n" + string(body)
}

// BuiltinSkillsForRole returns the list of builtin skill names for a given role,
// read dynamically from the role template's YAML frontmatter.
func BuiltinSkillsForRole(role string) []string {
	if role == "" {
		return []string{"gitea-api"}
	}
	fileName := roleTemplateFile(role)
	if fileName == "" {
		return []string{"gitea-api"}
	}
	data, err := builtinFS.ReadFile("builtin-role/agents-templates/" + fileName)
	if err != nil {
		return []string{"gitea-api"}
	}
	fm, err := parseRoleFrontmatter(data)
	if err != nil || fm == nil || len(fm.Skills) == 0 {
		return []string{"gitea-api"}
	}
	return fm.Skills
}

// ReadSkill reads a skill file from the embedded skills filesystem.
func ReadSkill(name string) string {
	data, err := skillsFS.ReadFile("skills/" + name + "/SKILL.md")
	if err != nil {
		return "# " + name + " Skill\n(skill content not found)\n"
	}
	return string(data)
}

// roleTemplateFile returns the template filename for a role by checking
// if <role>.md exists in the embedded filesystem.
func roleTemplateFile(role string) string {
	if role == "" {
		return ""
	}
	name := role + ".md"
	if _, err := builtinFS.ReadFile("builtin-role/agents-templates/" + name); err != nil {
		return ""
	}
	return name
}

// ReadPromptForRole returns the role-specific PROMPT.md if it exists,
// otherwise falls back to the default PROMPT.md.
func ReadPromptForRole(role string) string {
	if role != "" {
		data, err := builtinFS.ReadFile("builtin-role/prompts/" + role + ".md")
		if err == nil {
			return string(data)
		}
	}
	return ReadPrompt()
}

// ListRoleTypes scans embedded agents-templates/*.md files (excluding base.md),
// parses their YAML frontmatter, and returns metadata for each discovered role type.
func ListRoleTypes() []k8sclient.RoleTypeMeta {
	dir := "builtin-role/agents-templates"
	entries, err := fs.ReadDir(builtinFS, dir)
	if err != nil {
		return nil
	}

	var types []k8sclient.RoleTypeMeta
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || name == "base.md" {
			continue
		}
		data, err := builtinFS.ReadFile(dir + "/" + name)
		if err != nil {
			continue
		}
		fm, err := parseRoleFrontmatter(data)
		if err != nil || fm == nil {
			continue
		}
		roleName := strings.TrimSuffix(name, ".md")
		types = append(types, k8sclient.RoleTypeMeta{
			Name:        roleName,
			Label:       fm.Label,
			Description: fm.Description,
			Skills:      fm.Skills,
			Permissions: fm.Permissions,
		})
	}
	return types
}
