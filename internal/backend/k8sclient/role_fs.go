package k8sclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/entropyGen/entropyGen/internal/common/giteaclient"
)

type RoleFile struct {
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Role struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Files       []RoleFile `json:"files"`
	FileCount   int        `json:"file_count"`
	AgentCount  int        `json:"agent_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateRoleRequest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Role         string            `json:"role,omitempty"`          // developer | reviewer | sre | observer | custom
	InitialFiles []string          `json:"initial_files,omitempty"` // well-known file names to create with builtin defaults
	Files        map[string]string `json:"files,omitempty"`         // explicit file contents (overrides builtin defaults)
}

// RoleTypeMeta describes a builtin role type discovered from YAML frontmatter.
type RoleTypeMeta struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Skills      []string `json:"skills"`
	Permissions []string `json:"permissions"`
}

// ValidationIssue represents a problem found during role validation.
type ValidationIssue struct {
	File    string `json:"file"`
	Level   string `json:"level"`   // "error" | "warning"
	Message string `json:"message"`
}

// BuiltinContentProvider supplies default content for role files.
type BuiltinContentProvider interface {
	ReadSOUL() string
	ReadPrompt() string
	ReadPromptForRole(role string) string
	BuildAgentsMD(role string) string
	BuiltinSkillsForRole(role string) []string
	ReadSkill(name string) string
	ListRoleTypes() []RoleTypeMeta
}

// roleMetadata is persisted as .metadata.json in each role directory.
type roleMetadata struct {
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const metadataFile = ".metadata.json"

type RoleClient struct {
	basePath    string
	agentClient *AgentClient
	builtin     BuiltinContentProvider
	giteaClient *giteaclient.Client
	k8s         kubernetes.Interface
	namespace   string
}

func NewRoleClient(basePath string, agentClient *AgentClient, builtin BuiltinContentProvider, giteaClient *giteaclient.Client, k8s kubernetes.Interface, namespace string) *RoleClient {
	return &RoleClient{basePath: basePath, agentClient: agentClient, builtin: builtin, giteaClient: giteaClient, k8s: k8s, namespace: namespace}
}

// validatePath ensures the path is safe: no absolute paths, no ".." traversal.
func validatePath(name string) error {
	cleaned := filepath.Clean(name)
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("absolute paths not allowed: %s", name)
	}
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return fmt.Errorf("path traversal not allowed: %s", name)
		}
	}
	return nil
}

func (r *RoleClient) roleDir(name string) string {
	return filepath.Join(r.basePath, name)
}

func (r *RoleClient) readMetadata(name string) (*roleMetadata, error) {
	data, err := os.ReadFile(filepath.Join(r.roleDir(name), metadataFile))
	if err != nil {
		return nil, err
	}
	var m roleMetadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse metadata for role %s: %w", name, err)
	}
	return &m, nil
}

func (r *RoleClient) writeMetadata(name string, m *roleMetadata) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.roleDir(name), metadataFile), data, 0644)
}

func (r *RoleClient) touchMetadata(name string) error {
	m, err := r.readMetadata(name)
	if err != nil {
		return err
	}
	m.UpdatedAt = time.Now().UTC()
	return r.writeMetadata(name, m)
}

func (r *RoleClient) List(ctx context.Context) ([]Role, error) {
	entries, err := os.ReadDir(r.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Role{}, nil
		}
		return nil, fmt.Errorf("list roles: %w", err)
	}
	roles := make([]Role, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		m, err := r.readMetadata(name)
		if err != nil {
			continue // skip directories without valid metadata
		}
		agentCount, _ := r.countAgentsForRole(ctx, name)
		fileCount := r.countFiles(name)
		roles = append(roles, Role{
			Name:        name,
			Description: m.Description,
			FileCount:   fileCount,
			AgentCount:  agentCount,
			CreatedAt:   m.CreatedAt,
			UpdatedAt:   m.UpdatedAt,
		})
	}
	return roles, nil
}

func (r *RoleClient) Get(ctx context.Context, name string) (*Role, error) {
	if err := validatePath(name); err != nil {
		return nil, err
	}
	m, err := r.readMetadata(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("role %s not found", name)
		}
		return nil, fmt.Errorf("get role %s: %w", name, err)
	}
	files, err := r.listFilesInternal(name)
	if err != nil {
		return nil, err
	}
	agentCount, _ := r.countAgentsForRole(ctx, name)
	return &Role{
		Name:        name,
		Description: m.Description,
		Files:       files,
		FileCount:   len(files),
		AgentCount:  agentCount,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}, nil
}

func (r *RoleClient) Create(ctx context.Context, req CreateRoleRequest) (*Role, error) {
	if err := validatePath(req.Name); err != nil {
		return nil, err
	}
	roleDir := r.roleDir(req.Name)
	if _, err := os.Stat(roleDir); err == nil {
		return nil, fmt.Errorf("role %s already exists", req.Name)
	}
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		return nil, fmt.Errorf("create role dir %s: %w", req.Name, err)
	}

	// Collect all files to write
	data := make(map[string]string, 8)

	// Step 1: Populate builtin defaults based on role type
	if r.builtin != nil && req.Role != "" && req.Role != "custom" {
		wellKnown := []string{"SOUL.md", "PROMPT.md", "AGENTS.md"}
		for _, f := range wellKnown {
			content := r.builtinContentFor(f, req.Role)
			if content != "" {
				data[f] = content
			}
		}
		for _, skill := range r.builtin.BuiltinSkillsForRole(req.Role) {
			key := "skills/" + skill + "/SKILL.md"
			data[key] = r.builtin.ReadSkill(skill)
		}
	}

	// Step 2: InitialFiles
	for _, f := range req.InitialFiles {
		if _, exists := data[f]; !exists {
			data[f] = r.builtinContentFor(f, req.Role)
		}
	}

	// Step 3: User-supplied files override everything
	for k, v := range req.Files {
		data[k] = v
	}

	// Write all files
	for path, content := range data {
		if err := validatePath(path); err != nil {
			_ = os.RemoveAll(roleDir)
			return nil, err
		}
		fullPath := filepath.Join(roleDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			_ = os.RemoveAll(roleDir)
			return nil, fmt.Errorf("create dir for %s: %w", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			_ = os.RemoveAll(roleDir)
			return nil, fmt.Errorf("write file %s: %w", path, err)
		}
	}

	// Write metadata
	now := time.Now().UTC()
	m := &roleMetadata{
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.writeMetadata(req.Name, m); err != nil {
		_ = os.RemoveAll(roleDir)
		return nil, fmt.Errorf("write metadata for %s: %w", req.Name, err)
	}

	return r.Get(ctx, req.Name)
}

func (r *RoleClient) builtinContentFor(filename, role string) string {
	if r.builtin == nil {
		return ""
	}
	switch filename {
	case "SOUL.md":
		return r.builtin.ReadSOUL()
	case "PROMPT.md":
		return r.builtin.ReadPromptForRole(role)
	case "AGENTS.md":
		return r.builtin.BuildAgentsMD(role)
	default:
		return ""
	}
}

func (r *RoleClient) UpdateDescription(ctx context.Context, name, description string) (*Role, error) {
	if err := validatePath(name); err != nil {
		return nil, err
	}
	m, err := r.readMetadata(name)
	if err != nil {
		return nil, fmt.Errorf("get role %s for update: %w", name, err)
	}
	m.Description = description
	m.UpdatedAt = time.Now().UTC()
	if err := r.writeMetadata(name, m); err != nil {
		return nil, fmt.Errorf("update description for role %s: %w", name, err)
	}
	return r.Get(ctx, name)
}

func (r *RoleClient) Delete(ctx context.Context, name string) error {
	if err := validatePath(name); err != nil {
		return err
	}
	agentCount, err := r.countAgentsForRole(ctx, name)
	if err != nil {
		return fmt.Errorf("count agents for role %s: %w", name, err)
	}
	if agentCount > 0 {
		return fmt.Errorf("%d agents are using this role", agentCount)
	}
	roleDir := r.roleDir(name)
	if _, err := os.Stat(roleDir); os.IsNotExist(err) {
		return fmt.Errorf("role %s not found", name)
	}
	if err := os.RemoveAll(roleDir); err != nil {
		return fmt.Errorf("remove role dir %s: %w", name, err)
	}

	// Clean up the role-level Gitea user (purge=true handles repo ownership).
	giteaUsername := "role-" + name
	if r.giteaClient != nil {
		if err := r.giteaClient.DeleteUser(ctx, giteaUsername); err != nil {
			// Log but don't fail — Gitea user may not exist if role was never reconciled.
			_ = err
		}
	}

	// Clean up the role-level Gitea token Secret.
	if r.k8s != nil && r.namespace != "" {
		secretName := fmt.Sprintf("role-%s-gitea-token", name)
		err := r.k8s.CoreV1().Secrets(r.namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete gitea token secret %q: %w", secretName, err)
		}
	}

	return nil
}

func (r *RoleClient) ListFiles(ctx context.Context, roleName string) ([]RoleFile, error) {
	if err := validatePath(roleName); err != nil {
		return nil, err
	}
	return r.listFilesInternal(roleName)
}

func (r *RoleClient) listFilesInternal(roleName string) ([]RoleFile, error) {
	roleDir := r.roleDir(roleName)
	if _, err := os.Stat(roleDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("role %s not found", roleName)
	}

	var files []RoleFile
	err := filepath.WalkDir(roleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(roleDir, path)
		rel = filepath.ToSlash(rel)
		if rel == metadataFile {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, RoleFile{
			Name:      rel,
			Content:   string(content),
			UpdatedAt: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list files in role %s: %w", roleName, err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func (r *RoleClient) GetFile(ctx context.Context, roleName, filename string) (*RoleFile, error) {
	if err := validatePath(roleName); err != nil {
		return nil, err
	}
	if err := validatePath(filename); err != nil {
		return nil, err
	}
	fullPath := filepath.Join(r.roleDir(roleName), filename)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file %q not found in role %s", filename, roleName)
		}
		return nil, err
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	return &RoleFile{
		Name:      filename,
		Content:   string(content),
		UpdatedAt: info.ModTime(),
	}, nil
}

func (r *RoleClient) PutFile(ctx context.Context, roleName, filename, content string) (*RoleFile, error) {
	if err := validatePath(roleName); err != nil {
		return nil, err
	}
	if err := validatePath(filename); err != nil {
		return nil, err
	}
	roleDir := r.roleDir(roleName)
	if _, err := os.Stat(filepath.Join(roleDir, metadataFile)); os.IsNotExist(err) {
		return nil, fmt.Errorf("role %s not found", roleName)
	}
	fullPath := filepath.Join(roleDir, filename)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("create dir for %s: %w", filename, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("put file %q in role %s: %w", filename, roleName, err)
	}
	if err := r.touchMetadata(roleName); err != nil {
		return nil, fmt.Errorf("update metadata for role %s: %w", roleName, err)
	}
	info, _ := os.Stat(fullPath)
	return &RoleFile{
		Name:      filename,
		Content:   content,
		UpdatedAt: info.ModTime(),
	}, nil
}

func (r *RoleClient) DeleteFile(ctx context.Context, roleName, filename string) error {
	if err := validatePath(roleName); err != nil {
		return err
	}
	if err := validatePath(filename); err != nil {
		return err
	}
	fullPath := filepath.Join(r.roleDir(roleName), filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("file %q not found in role %s", filename, roleName)
	}
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("delete file %q from role %s: %w", filename, roleName, err)
	}
	// Clean up empty parent directories (but not the role dir itself)
	r.cleanEmptyParents(filepath.Dir(fullPath), r.roleDir(roleName))
	if err := r.touchMetadata(roleName); err != nil {
		return fmt.Errorf("update metadata for role %s: %w", roleName, err)
	}
	return nil
}

func (r *RoleClient) RenameFile(ctx context.Context, roleName, oldName, newName string) (*RoleFile, error) {
	if err := validatePath(roleName); err != nil {
		return nil, err
	}
	if err := validatePath(oldName); err != nil {
		return nil, err
	}
	if err := validatePath(newName); err != nil {
		return nil, err
	}
	roleDir := r.roleDir(roleName)
	oldPath := filepath.Join(roleDir, oldName)
	newPath := filepath.Join(roleDir, newName)
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file %q not found in role %s", oldName, roleName)
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return nil, fmt.Errorf("create dir for %s: %w", newName, err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, fmt.Errorf("rename file %q to %q in role %s: %w", oldName, newName, roleName, err)
	}
	r.cleanEmptyParents(filepath.Dir(oldPath), roleDir)
	if err := r.touchMetadata(roleName); err != nil {
		return nil, fmt.Errorf("update metadata for role %s: %w", roleName, err)
	}
	content, _ := os.ReadFile(newPath)
	info, _ := os.Stat(newPath)
	return &RoleFile{
		Name:      newName,
		Content:   string(content),
		UpdatedAt: info.ModTime(),
	}, nil
}

// cleanEmptyParents removes empty directories from dir up to (but not including) stopAt.
func (r *RoleClient) cleanEmptyParents(dir, stopAt string) {
	for dir != stopAt && dir != "." && dir != "/" {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}

func (r *RoleClient) countAgentsForRole(ctx context.Context, roleName string) (int, error) {
	if r.agentClient == nil {
		return 0, nil
	}
	agents, err := r.agentClient.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list agents: %w", err)
	}
	count := 0
	for i := range agents {
		if agents[i].Spec.Role == roleName {
			count++
		}
	}
	return count, nil
}

// countFiles returns the number of non-metadata files in a role directory.
func (r *RoleClient) countFiles(roleName string) int {
	roleDir := r.roleDir(roleName)
	count := 0
	_ = filepath.WalkDir(roleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(roleDir, path)
		if filepath.ToSlash(rel) != metadataFile {
			count++
		}
		return nil
	})
	return count
}

// ListRoleTypes delegates to the builtin content provider.
func (r *RoleClient) ListRoleTypes() []RoleTypeMeta {
	if r.builtin == nil {
		return nil
	}
	return r.builtin.ListRoleTypes()
}

// ValidateRole checks a role's files for common issues.
func (r *RoleClient) ValidateRole(ctx context.Context, name string) ([]ValidationIssue, error) {
	if err := validatePath(name); err != nil {
		return nil, err
	}
	files, err := r.listFilesInternal(name)
	if err != nil {
		return nil, err
	}
	fileMap := make(map[string]string, len(files))
	for _, f := range files {
		fileMap[f.Name] = f.Content
	}

	var issues []ValidationIssue

	// Check well-known files exist
	for _, wk := range []string{"SOUL.md", "PROMPT.md", "AGENTS.md"} {
		if _, ok := fileMap[wk]; !ok {
			issues = append(issues, ValidationIssue{
				File:    wk,
				Level:   "warning",
				Message: wk + " is missing",
			})
		}
	}

	// Check template placeholders in PROMPT.md
	if content, ok := fileMap["PROMPT.md"]; ok {
		for _, ph := range []string{"{{REPOS}}", "{{AGENT_ID}}"} {
			if !strings.Contains(content, ph) {
				issues = append(issues, ValidationIssue{
					File:    "PROMPT.md",
					Level:   "warning",
					Message: "Missing placeholder " + ph,
				})
			}
		}
	}

	// Check skill files follow convention
	for _, f := range files {
		if strings.HasPrefix(f.Name, "skills/") {
			parts := strings.Split(f.Name, "/")
			if len(parts) != 3 || parts[2] != "SKILL.md" {
				issues = append(issues, ValidationIssue{
					File:    f.Name,
					Level:   "warning",
					Message: "Skill files should follow skills/<name>/SKILL.md convention",
				})
			}
		}
	}

	return issues, nil
}
