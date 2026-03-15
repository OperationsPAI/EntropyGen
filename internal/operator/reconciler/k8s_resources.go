package reconciler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

const (
	defaultStorageSize = "1Gi"
	annotationCfgHash  = "aidevops.io/config-hash"
)

// roleData holds parsed content from a Role ConfigMap (role-{name}).
type roleData struct {
	Soul       string            // SOUL.md content
	Prompt     string            // PROMPT.md content (injected into cron prompt)
	AgentsMD   string            // AGENTS.md content (overrides template)
	Skills     map[string]string // skills/* files
	ExtraFiles map[string]string // other custom files
}

func agentRuntimeImageName() string {
	if v := os.Getenv("AGENT_RUNTIME_IMAGE"); v != "" {
		return v
	}
	return "registry.local/agent-runtime:latest"
}

func agentRuntimeImage(agent *agentapi.Agent) string {
	if agent.Spec.RuntimeImage != "" {
		return agent.Spec.RuntimeImage
	}
	return agentRuntimeImageName()
}

// readRoleDataFromDir reads a role directory and classifies files into well-known role data.
// Well-known file names are matched case-insensitively. .metadata.json is skipped.
func readRoleDataFromDir(roleDir string) (*roleData, error) {
	rd := &roleData{
		Skills:     map[string]string{},
		ExtraFiles: map[string]string{},
	}
	err := filepath.WalkDir(roleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(roleDir, path)
		rel = filepath.ToSlash(rel)
		if rel == ".metadata.json" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(rel)
		switch {
		case lower == "soul.md":
			rd.Soul = string(content)
		case lower == "prompt.md":
			rd.Prompt = string(content)
		case lower == "agents.md":
			rd.AgentsMD = string(content)
		case strings.HasPrefix(lower, "skills/"):
			rd.Skills[rel] = string(content)
		default:
			rd.ExtraFiles[rel] = string(content)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read role dir %s: %w", roleDir, err)
	}
	return rd, nil
}

// fetchRoleData reads the role directory from PVC and parses its content.
// Returns nil if the directory does not exist (graceful degradation).
func (r *ResourceReconciler) fetchRoleData(ctx context.Context, agent *agentapi.Agent) (*roleData, error) {
	if agent.Spec.Role == "" {
		return nil, nil
	}
	roleDir := filepath.Join(r.RolesDataPath, agent.Spec.Role)
	if _, err := os.Stat(roleDir); os.IsNotExist(err) {
		return nil, nil
	}
	return readRoleDataFromDir(roleDir)
}
// K8s ConfigMap keys cannot contain '/'.
func skillKey(path string) string {
	return strings.ReplaceAll(path, "/", "__")
}

// EnsureConfigMap creates or updates the main agent ConfigMap (openclaw.json, SOUL.md, AGENTS.md).
func (r *ResourceReconciler) EnsureConfigMap(ctx context.Context, agent *agentapi.Agent) error {
	data := buildConfigMapData(agent, r.roleData, r.GatewayURL, r.LLMBaseURL, r.LLMAPIKey)
	hash := computeHash(data)
	name := fmt.Sprintf("agent-%s-config", agent.Name)

	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: agent.Namespace}, cm)
	if errors.IsNotFound(err) {
		newCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   agent.Namespace,
				Annotations: map[string]string{annotationCfgHash: hash},
			},
			Data: data,
		}
		_ = controllerutil.SetControllerReference(agent, newCM, r.Scheme)
		return r.Client.Create(ctx, newCM)
	}
	if err != nil {
		return err
	}
	cm.Data = data
	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
	}
	cm.Annotations[annotationCfgHash] = hash
	return r.Client.Update(ctx, cm)
}

func buildConfigMapData(agent *agentapi.Agent, rd *roleData, gatewayURL, llmBaseURL, llmAPIKey string) map[string]string {
	model := ""
	if agent.Spec.LLM != nil && agent.Spec.LLM.Model != "" {
		model = agent.Spec.LLM.Model
	}

	// Extract provider prefix (e.g. "litellm" from "litellm/MiniMax-M2.5")
	providerName := "litellm"
	modelID := model
	if idx := strings.Index(model, "/"); idx > 0 {
		providerName = model[:idx]
		modelID = model[idx+1:]
	} else {
		// No provider prefix — default to litellm and add prefix
		model = providerName + "/" + model
	}

	// Build openclaw.json using the new config format (models.providers + agents.defaults)
	openclawCfg := map[string]interface{}{
		"models": map[string]interface{}{
			"providers": map[string]interface{}{
				providerName: map[string]interface{}{
					"baseUrl": llmBaseURL,
					"apiKey":  llmAPIKey,
					"api":     "openai-completions",
					"models": []map[string]interface{}{
						{
							"id":            modelID,
							"name":          modelID,
							"reasoning":     false,
							"input":         []string{"text"},
							"contextWindow": 128000,
							"maxTokens":     32000,
						},
					},
				},
			},
		},
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"model": map[string]interface{}{
					"primary": model,
				},
			},
		},
		"gateway": map[string]interface{}{
			"controlUi": map[string]interface{}{
				"dangerouslyAllowHostHeaderOriginFallback": true,
			},
			"http": map[string]interface{}{
				"endpoints": map[string]interface{}{
					"chatCompletions": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
	}
	openclawJSON, _ := json.MarshalIndent(openclawCfg, "", "  ")

	// SOUL.md: Role data is the single source of truth
	soul := ""
	if rd != nil && rd.Soul != "" {
		soul = rd.Soul
	}

	// AGENTS.md: Role data is the single source of truth
	agentsMD := ""
	if rd != nil && rd.AgentsMD != "" {
		agentsMD = rd.AgentsMD
	}

	result := map[string]string{
		"openclaw.json": string(openclawJSON),
		"SOUL.md":       soul,
		"AGENTS.md":     agentsMD,
	}

	// PROMPT.md: include for observability (cron reads from roleData directly)
	if rd != nil && rd.Prompt != "" {
		prompt := rd.Prompt
		// Replace template variables in PROMPT.md
		if agent.Spec.Gitea != nil && len(agent.Spec.Gitea.Repos) > 0 {
			prompt = strings.ReplaceAll(prompt, "{{REPOS}}", strings.Join(agent.Spec.Gitea.Repos, ","))
		}
		prompt = strings.ReplaceAll(prompt, "{{AGENT_ID}}", "agent-"+agent.Name)
		prompt = strings.ReplaceAll(prompt, "{{AGENT_ROLE}}", agent.Spec.Role)
		result["PROMPT.md"] = prompt
	}

	return result
}

func computeHash(data map[string]string) string {
	h := sha256.New()
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k + "=" + data[k] + "\n"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// EnsureSkillsConfigMap creates or updates the skills ConfigMap.
func (r *ResourceReconciler) EnsureSkillsConfigMap(ctx context.Context, agent *agentapi.Agent) error {
	data := buildSkillsData(agent, r.roleData)
	name := fmt.Sprintf("agent-%s-skills", agent.Name)

	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: agent.Namespace}, cm)
	if errors.IsNotFound(err) {
		newCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: agent.Namespace},
			Data:       data,
		}
		_ = controllerutil.SetControllerReference(agent, newCM, r.Scheme)
		return r.Client.Create(ctx, newCM)
	}
	if err != nil {
		return err
	}
	cm.Data = data
	return r.Client.Update(ctx, cm)
}

// EnsureRoleFilesConfigMap creates a ConfigMap for role-specific extra files.
// Skipped if the role has no extra files.
func (r *ResourceReconciler) EnsureRoleFilesConfigMap(ctx context.Context, agent *agentapi.Agent) error {
	if r.roleData == nil || len(r.roleData.ExtraFiles) == 0 {
		return nil
	}
	name := fmt.Sprintf("agent-%s-role-files", agent.Name)

	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: agent.Namespace}, cm)
	if errors.IsNotFound(err) {
		newCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: agent.Namespace},
			Data:       r.roleData.ExtraFiles,
		}
		_ = controllerutil.SetControllerReference(agent, newCM, r.Scheme)
		return r.Client.Create(ctx, newCM)
	}
	if err != nil {
		return err
	}
	cm.Data = r.roleData.ExtraFiles
	return r.Client.Update(ctx, cm)
}

func buildSkillsData(agent *agentapi.Agent, rd *roleData) map[string]string {
	data := map[string]string{}

	// Skills come from the Role directory. Keys use "/" paths (e.g. "skills/gitea-api/SKILL.md").
	// Convert to "__" format for ConfigMap keys (e.g. "skills__gitea-api__SKILL.md").
	if rd != nil {
		for k, v := range rd.Skills {
			data[skillKey(k)] = v
		}
	}

	return data
}

// buildSkillItems returns the ConfigMap items mapping for the skills volume,
// mapping ConfigMap keys (e.g. "skills__gitea-api__SKILL.md") to mount paths (e.g. "skills/gitea-api/SKILL.md").
func buildSkillItems(agent *agentapi.Agent, rd *roleData) []corev1.KeyToPath {
	if rd == nil {
		return nil
	}
	// rd.Skills keys use "/" paths (from filesystem).
	// Collect and sort the paths.
	var skillPaths []string
	for k := range rd.Skills {
		skillPaths = append(skillPaths, k)
	}
	sort.Strings(skillPaths)

	items := make([]corev1.KeyToPath, 0, len(skillPaths))
	for _, p := range skillPaths {
		items = append(items, corev1.KeyToPath{Key: skillKey(p), Path: p})
	}
	return items
}

// EnsurePVC creates the workspace PVC. PVCs are not updated once created.
func (r *ResourceReconciler) EnsurePVC(ctx context.Context, agent *agentapi.Agent) error {
	name := fmt.Sprintf("agent-%s-workspace", agent.Name)
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: agent.Namespace}, pvc)
	if err == nil {
		return nil // PVCs are immutable once bound
	}
	if !errors.IsNotFound(err) {
		return err
	}

	storageSize := defaultStorageSize
	var storageClass *string
	if agent.Spec.Memory != nil {
		if agent.Spec.Memory.StorageSize != "" {
			storageSize = agent.Spec.Memory.StorageSize
		}
		if agent.Spec.Memory.StorageClass != "" {
			sc := agent.Spec.Memory.StorageClass
			storageClass = &sc
		}
	}
	// Fall back to platform default storage class
	if storageClass == nil && r.DefaultStorageClass != "" {
		sc := r.DefaultStorageClass
		storageClass = &sc
	}

	newPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: agent.Namespace},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
			StorageClassName: storageClass,
		},
	}
	_ = controllerutil.SetControllerReference(agent, newPVC, r.Scheme)
	return r.Client.Create(ctx, newPVC)
}

// EnsureService creates a ClusterIP Service for the agent pod so the backend
// can proxy observe requests to the observer sidecar on port 8081.
func (r *ResourceReconciler) EnsureService(ctx context.Context, agent *agentapi.Agent) error {
	name := fmt.Sprintf("agent-%s", agent.Name)
	svc := &corev1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: agent.Namespace}, svc)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	newSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: agent.Namespace},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name},
			Ports: []corev1.ServicePort{
				{Name: "observe", Port: 8081, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	_ = controllerutil.SetControllerReference(agent, newSvc, r.Scheme)
	return r.Client.Create(ctx, newSvc)
}

// EnsureServiceAccount creates the agent ServiceAccount.
func (r *ResourceReconciler) EnsureServiceAccount(ctx context.Context, agent *agentapi.Agent) error {
	name := fmt.Sprintf("agent-%s", agent.Name)
	sa := &corev1.ServiceAccount{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: agent.Namespace}, sa)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	newSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: agent.Namespace},
	}
	_ = controllerutil.SetControllerReference(agent, newSA, r.Scheme)
	return r.Client.Create(ctx, newSA)
}

// EnsureRoleBinding creates RoleBindings for the agent.
//   - Home namespace: binds to kubernetes.rbacRole (default: "agent-readonly-role" ClusterRole)
//   - Extra namespaces: for each entry in kubernetes.namespaceAccess, creates a
//     RoleBinding in that namespace (skipped if namespace doesn't exist yet).
//     The rbacRole must exist as a Role in the target namespace.
func (r *ResourceReconciler) EnsureRoleBinding(ctx context.Context, agent *agentapi.Agent) error {
	saName := fmt.Sprintf("agent-%s", agent.Name)
	rbName := fmt.Sprintf("agent-%s", agent.Name)

	// Determine the RBAC role to bind (CRD field → default).
	roleName := "agent-readonly-role"
	roleKind := "ClusterRole"
	if agent.Spec.Kubernetes != nil && agent.Spec.Kubernetes.RBACRole != "" {
		roleName = agent.Spec.Kubernetes.RBACRole
	}

	// Home namespace RoleBinding (ClusterRole binding).
	rb := &rbacv1.RoleBinding{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: rbName, Namespace: agent.Namespace}, rb)
	if errors.IsNotFound(err) {
		newRB := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: agent.Namespace},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     roleKind,
				Name:     roleName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: agent.Namespace,
			}},
		}
		_ = controllerutil.SetControllerReference(agent, newRB, r.Scheme)
		if err := r.Client.Create(ctx, newRB); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Extra namespace RoleBindings (driven by kubernetes.namespaceAccess).
	if agent.Spec.Kubernetes == nil {
		return nil
	}
	for _, ns := range agent.Spec.Kubernetes.NamespaceAccess {
		// Skip if target namespace doesn't exist yet.
		nsObj := &corev1.Namespace{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: ns}, nsObj); errors.IsNotFound(err) {
			continue
		} else if err != nil {
			return err
		}

		extraRBName := fmt.Sprintf("agent-%s-%s", agent.Name, ns)
		extraRB := &rbacv1.RoleBinding{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: extraRBName, Namespace: ns}, extraRB); errors.IsNotFound(err) {
			newRB := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: extraRBName, Namespace: ns},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     roleName,
				},
				Subjects: []rbacv1.Subject{{
					Kind:      "ServiceAccount",
					Name:      saName,
					Namespace: agent.Namespace,
				}},
			}
			if err := r.Client.Create(ctx, newRB); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

// EnsureDeployment creates or updates the agent Deployment.
// Config hash annotation on pod template triggers rolling restart when ConfigMap changes.
func (r *ResourceReconciler) EnsureDeployment(ctx context.Context, agent *agentapi.Agent) error {
	// Get current config hash to propagate to pod template
	cmName := fmt.Sprintf("agent-%s-config", agent.Name)
	cm := &corev1.ConfigMap{}
	cfgHash := ""
	if err := r.Client.Get(ctx, types.NamespacedName{Name: cmName, Namespace: agent.Namespace}, cm); err == nil {
		if cm.Annotations != nil {
			cfgHash = cm.Annotations[annotationCfgHash]
		}
	}

	desired := buildDeployment(agent, r.roleData, cfgHash, r.GatewayURL, r.RedisAddr, r.GiteaURL)
	_ = controllerutil.SetControllerReference(agent, desired, r.Scheme)

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if errors.IsNotFound(err) {
		return r.Client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update mutable fields
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template.Spec = desired.Spec.Template.Spec
	if existing.Spec.Template.Annotations == nil {
		existing.Spec.Template.Annotations = map[string]string{}
	}
	existing.Spec.Template.Annotations[annotationCfgHash] = cfgHash
	return r.Client.Update(ctx, existing)
}

func buildDeployment(agent *agentapi.Agent, rd *roleData, cfgHash string, gatewayURL string, redisAddr string, giteaURL string) *appsv1.Deployment {
	replicas := int32(1)
	if agent.Spec.Paused {
		replicas = 0
	}

	saName := fmt.Sprintf("agent-%s", agent.Name)
	configCMName := fmt.Sprintf("agent-%s-config", agent.Name)
	skillsCMName := fmt.Sprintf("agent-%s-skills", agent.Name)
	jwtSecretName := fmt.Sprintf("agent-%s-jwt-token", agent.Name)
	giteaTokenSecretName := fmt.Sprintf("agent-%s-gitea-token", agent.Name)
	pvcName := fmt.Sprintf("agent-%s-workspace", agent.Name)

	model := ""
	if agent.Spec.LLM != nil && agent.Spec.LLM.Model != "" {
		model = agent.Spec.LLM.Model
	}

	agentRepos := ""
	if agent.Spec.Gitea != nil && len(agent.Spec.Gitea.Repos) > 0 {
		agentRepos = strings.Join(agent.Spec.Gitea.Repos, ",")
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "config", MountPath: "/agent/config", ReadOnly: true},
		{Name: "skills", MountPath: "/agent/skills", ReadOnly: true},
		{Name: "jwt-token", MountPath: "/agent/secrets/jwt-token", SubPath: "token", ReadOnly: true},
		{Name: "gitea-token", MountPath: "/agent/secrets/gitea-token", SubPath: "token", ReadOnly: true},
		{Name: "workspace", MountPath: "/home/node/.openclaw/workspace"},
	}

	volumes := []corev1.Volume{
		{Name: "config", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configCMName},
			},
		}},
		{Name: "skills", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: skillsCMName},
				Items:                buildSkillItems(agent, rd),
			},
		}},
		{Name: "jwt-token", VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: jwtSecretName},
		}},
		{Name: "gitea-token", VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: giteaTokenSecretName},
		}},
		{Name: "workspace", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		}},
	}

	// Add role-files volume if role has extra files
	if rd != nil && len(rd.ExtraFiles) > 0 {
		roleFilesCMName := fmt.Sprintf("agent-%s-role-files", agent.Name)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "role-files", MountPath: "/agent/role", ReadOnly: true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "role-files", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: roleFilesCMName},
				},
			},
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("agent-%s", agent.Name),
			Namespace: agent.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "agent-" + agent.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              "agent-" + agent.Name,
						"aidevops.io/role": agent.Spec.Role,
					},
					Annotations: map[string]string{
						annotationCfgHash: cfgHash,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: saName,
					Containers: []corev1.Container{{
						Name:      "agent-runtime",
						Image:     agentRuntimeImage(agent),
						Resources: buildResourceRequirements(agent),
						Env: []corev1.EnvVar{
							{Name: "AGENT_ID", Value: "agent-" + agent.Name},
							{Name: "AGENT_ROLE", Value: agent.Spec.Role},
							{Name: "AGENT_REPOS", Value: agentRepos},
							{Name: "GATEWAY_URL", Value: gatewayURL},
							{Name: "LLM_MODEL", Value: model},
							{Name: "REDIS_ADDR", Value: redisAddr},
							{Name: "GITEA_BASE_URL", Value: giteaURL},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt32(8080),
								},
							},
							PeriodSeconds:    30,
							FailureThreshold: 3,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromInt32(8080),
								},
							},
							PeriodSeconds:    15,
							FailureThreshold: 2,
						},
						VolumeMounts: volumeMounts,
					}},
					Volumes: volumes,
				},
			},
		},
	}
}

func buildResourceRequirements(agent *agentapi.Agent) corev1.ResourceRequirements {
	if agent.Spec.Resources == nil {
		return corev1.ResourceRequirements{}
	}
	req := corev1.ResourceRequirements{}
	if r := agent.Spec.Resources.Requests; r != nil {
		req.Requests = corev1.ResourceList{}
		if r.CPU != "" {
			req.Requests[corev1.ResourceCPU] = resource.MustParse(r.CPU)
		}
		if r.Memory != "" {
			req.Requests[corev1.ResourceMemory] = resource.MustParse(r.Memory)
		}
	}
	if l := agent.Spec.Resources.Limits; l != nil {
		req.Limits = corev1.ResourceList{}
		if l.CPU != "" {
			req.Limits[corev1.ResourceCPU] = resource.MustParse(l.CPU)
		}
		if l.Memory != "" {
			req.Limits[corev1.ResourceMemory] = resource.MustParse(l.Memory)
		}
	}
	return req
}
