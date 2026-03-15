package k8sclient

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	AgentCount  int        `json:"agent_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateRoleRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Role         string   `json:"role,omitempty"`          // developer | reviewer | sre | observer | custom
	InitialFiles []string `json:"initial_files,omitempty"`
}

// BuiltinContentProvider supplies default content for role files.
// Injected by the handler layer to avoid circular dependencies.
type BuiltinContentProvider interface {
	ReadSOUL() string
	ReadPrompt() string
	BuildAgentsMD(role string) string
	BuiltinSkillsForRole(role string) []string
	ReadSkill(name string) string
	SkillKey(path string) string
}

type RoleClient struct {
	k8s         kubernetes.Interface
	agentClient *AgentClient
	namespace   string
	builtin     BuiltinContentProvider
}

func NewRoleClient(k8s kubernetes.Interface, agentClient *AgentClient, namespace string, builtin BuiltinContentProvider) *RoleClient {
	return &RoleClient{k8s: k8s, agentClient: agentClient, namespace: namespace, builtin: builtin}
}

func (r *RoleClient) ensureClient() error {
	if r.k8s == nil {
		return fmt.Errorf("k8s client not available")
	}
	return nil
}

func (r *RoleClient) List(ctx context.Context) ([]Role, error) {
	if err := r.ensureClient(); err != nil {
		return nil, err
	}
	cmList, err := r.k8s.CoreV1().ConfigMaps(r.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "entropygen.io/component=role",
	})
	if err != nil {
		return nil, fmt.Errorf("list role configmaps: %w", err)
	}
	roles := make([]Role, 0, len(cmList.Items))
	for i := range cmList.Items {
		roleName := cmList.Items[i].Name[len("role-"):]
		agentCount, err := r.countAgentsForRole(ctx, roleName)
		if err != nil {
			return nil, fmt.Errorf("count agents for role %s: %w", roleName, err)
		}
		roles = append(roles, r.cmToRole(&cmList.Items[i], agentCount))
	}
	return roles, nil
}

func (r *RoleClient) Get(ctx context.Context, name string) (*Role, error) {
	if err := r.ensureClient(); err != nil {
		return nil, err
	}
	cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, cmName(name), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get role %s: %w", name, err)
	}
	agentCount, err := r.countAgentsForRole(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("count agents for role %s: %w", name, err)
	}
	role := r.cmToRole(cm, agentCount)
	return &role, nil
}

func (r *RoleClient) Create(ctx context.Context, req CreateRoleRequest) (*Role, error) {
	if err := r.ensureClient(); err != nil {
		return nil, err
	}

	data := make(map[string]string, len(req.InitialFiles)+4)

	// Inject builtin content for each initial file based on the role type
	for _, f := range req.InitialFiles {
		data[f] = r.builtinContentFor(f, req.Role)
	}

	// Inject builtin skills for the role type
	if r.builtin != nil && req.Role != "" && req.Role != "custom" {
		for _, skill := range r.builtin.BuiltinSkillsForRole(req.Role) {
			key := r.builtin.SkillKey(skill + "/SKILL.md")
			if _, exists := data[key]; !exists {
				data[key] = r.builtin.ReadSkill(skill)
			}
		}
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName(req.Name),
			Namespace: r.namespace,
			Labels: map[string]string{
				"entropygen.io/component": "role",
			},
			Annotations: map[string]string{
				"entropygen.io/description": req.Description,
			},
		},
		Data: data,
	}
	created, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("role %s already exists: %w", req.Name, err)
		}
		return nil, fmt.Errorf("create role %s: %w", req.Name, err)
	}
	role := r.cmToRole(created, 0)
	return &role, nil
}

// builtinContentFor returns default builtin content for a well-known file.
func (r *RoleClient) builtinContentFor(filename, role string) string {
	if r.builtin == nil {
		return ""
	}
	switch filename {
	case "SOUL.md":
		return r.builtin.ReadSOUL()
	case "PROMPT.md":
		return r.builtin.ReadPrompt()
	case "AGENTS.md":
		return r.builtin.BuildAgentsMD(role)
	default:
		return ""
	}
}

func (r *RoleClient) UpdateDescription(ctx context.Context, name, description string) (*Role, error) {
	if err := r.ensureClient(); err != nil {
		return nil, err
	}
	cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, cmName(name), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get role %s for update: %w", name, err)
	}
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations["entropygen.io/description"] = description
	updated, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("update description for role %s: %w", name, err)
	}
	agentCount, err := r.countAgentsForRole(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("count agents for role %s: %w", name, err)
	}
	role := r.cmToRole(updated, agentCount)
	return &role, nil
}

func (r *RoleClient) Delete(ctx context.Context, name string) error {
	if err := r.ensureClient(); err != nil {
		return err
	}
	agentCount, err := r.countAgentsForRole(ctx, name)
	if err != nil {
		return fmt.Errorf("count agents for role %s: %w", name, err)
	}
	if agentCount > 0 {
		return fmt.Errorf("%d agents are using this role", agentCount)
	}
	if err := r.k8s.CoreV1().ConfigMaps(r.namespace).Delete(ctx, cmName(name), metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete role %s: %w", name, err)
	}
	return nil
}

func (r *RoleClient) ListFiles(ctx context.Context, roleName string) ([]RoleFile, error) {
	if err := r.ensureClient(); err != nil {
		return nil, err
	}
	cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, cmName(roleName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get role %s: %w", roleName, err)
	}
	ts := cm.CreationTimestamp.Time
	names := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		names = append(names, k)
	}
	sort.Strings(names)
	files := make([]RoleFile, 0, len(names))
	for _, n := range names {
		files = append(files, RoleFile{
			Name:      n,
			UpdatedAt: ts,
		})
	}
	return files, nil
}

func (r *RoleClient) GetFile(ctx context.Context, roleName, filename string) (*RoleFile, error) {
	if err := r.ensureClient(); err != nil {
		return nil, err
	}
	cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, cmName(roleName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get role %s: %w", roleName, err)
	}
	content, ok := cm.Data[filename]
	if !ok {
		return nil, fmt.Errorf("file %q not found in role %s", filename, roleName)
	}
	return &RoleFile{
		Name:      filename,
		Content:   content,
		UpdatedAt: cm.CreationTimestamp.Time,
	}, nil
}

func (r *RoleClient) PutFile(ctx context.Context, roleName, filename, content string) (*RoleFile, error) {
	if err := r.ensureClient(); err != nil {
		return nil, err
	}
	cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, cmName(roleName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get role %s: %w", roleName, err)
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[filename] = content
	updated, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("put file %q in role %s: %w", filename, roleName, err)
	}
	return &RoleFile{
		Name:      filename,
		Content:   content,
		UpdatedAt: updated.CreationTimestamp.Time,
	}, nil
}

func (r *RoleClient) DeleteFile(ctx context.Context, roleName, filename string) error {
	if err := r.ensureClient(); err != nil {
		return err
	}
	cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, cmName(roleName), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get role %s: %w", roleName, err)
	}
	if _, ok := cm.Data[filename]; !ok {
		return fmt.Errorf("file %q not found in role %s", filename, roleName)
	}
	delete(cm.Data, filename)
	if _, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("delete file %q from role %s: %w", filename, roleName, err)
	}
	return nil
}

func (r *RoleClient) RenameFile(ctx context.Context, roleName, oldName, newName string) (*RoleFile, error) {
	if err := r.ensureClient(); err != nil {
		return nil, err
	}
	cm, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Get(ctx, cmName(roleName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get role %s: %w", roleName, err)
	}
	content, ok := cm.Data[oldName]
	if !ok {
		return nil, fmt.Errorf("file %q not found in role %s", oldName, roleName)
	}
	delete(cm.Data, oldName)
	cm.Data[newName] = content
	updated, err := r.k8s.CoreV1().ConfigMaps(r.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("rename file %q to %q in role %s: %w", oldName, newName, roleName, err)
	}
	return &RoleFile{
		Name:      newName,
		Content:   content,
		UpdatedAt: updated.CreationTimestamp.Time,
	}, nil
}

func cmName(roleName string) string {
	return "role-" + roleName
}

func (r *RoleClient) cmToRole(cm *corev1.ConfigMap, agentCount int) Role {
	description := ""
	if cm.Annotations != nil {
		description = cm.Annotations["entropygen.io/description"]
	}
	ts := cm.CreationTimestamp.Time
	names := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		names = append(names, k)
	}
	sort.Strings(names)
	files := make([]RoleFile, 0, len(names))
	for _, n := range names {
		files = append(files, RoleFile{
			Name:      n,
			Content:   cm.Data[n],
			UpdatedAt: ts,
		})
	}
	roleName := cm.Name[len("role-"):]
	return Role{
		Name:        roleName,
		Description: description,
		Files:       files,
		AgentCount:  agentCount,
		CreatedAt:   cm.CreationTimestamp.Time,
		UpdatedAt:   ts,
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
