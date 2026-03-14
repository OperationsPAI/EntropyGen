package reconciler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
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

// parseRoleData classifies ConfigMap data entries into well-known role files.
// Well-known file names are matched case-insensitively.
func parseRoleData(data map[string]string) *roleData {
	rd := &roleData{
		Skills:     map[string]string{},
		ExtraFiles: map[string]string{},
	}
	for k, v := range data {
		lower := strings.ToLower(k)
		switch {
		case lower == "soul.md":
			rd.Soul = v
		case lower == "prompt.md":
			rd.Prompt = v
		case lower == "agents.md":
			rd.AgentsMD = v
		case strings.HasPrefix(lower, "skills__") || strings.HasPrefix(lower, "skills/"):
			rd.Skills[k] = v
		default:
			rd.ExtraFiles[k] = v
		}
	}
	return rd
}

// fetchRoleData reads the role-{roleName} ConfigMap and parses its content.
// Returns nil if the ConfigMap does not exist (graceful degradation).
func (r *ResourceReconciler) fetchRoleData(ctx context.Context, agent *agentapi.Agent) (*roleData, error) {
	if agent.Spec.Role == "" {
		return nil, nil
	}
	cmName := fmt.Sprintf("role-%s", agent.Spec.Role)
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: cmName, Namespace: agent.Namespace}, cm)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetch role configmap %s: %w", cmName, err)
	}
	return parseRoleData(cm.Data), nil
}
// K8s ConfigMap keys cannot contain '/'.
func skillKey(path string) string {
	return strings.ReplaceAll(path, "/", "__")
}

// EnsureConfigMap creates or updates the main agent ConfigMap (openclaw.json, SOUL.md, AGENTS.md, cron-config.json).
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
		},
	}
	openclawJSON, _ := json.MarshalIndent(openclawCfg, "", "  ")

	cronCfg := map[string]interface{}{}
	if agent.Spec.Cron != nil {
		cronCfg["schedule"] = agent.Spec.Cron.Schedule
		cronCfg["prompt"] = agent.Spec.Cron.Prompt
	}
	// If cron prompt is empty but role has PROMPT.md, use it
	if cronCfg["prompt"] == nil || cronCfg["prompt"] == "" {
		if rd != nil && rd.Prompt != "" {
			cronCfg["prompt"] = rd.Prompt
		}
	}
	cronJSON, _ := json.MarshalIndent(cronCfg, "", "  ")

	// SOUL.md: roleData > spec.soul > empty
	soul := agent.Spec.Soul
	if rd != nil && rd.Soul != "" {
		soul = rd.Soul
	}

	// AGENTS.md: roleData > template
	agentsMD := buildAgentsMD(agent.Spec.Role)
	if rd != nil && rd.AgentsMD != "" {
		agentsMD = rd.AgentsMD
	}

	return map[string]string{
		"openclaw.json":    string(openclawJSON),
		"SOUL.md":          soul,
		"AGENTS.md":        agentsMD,
		"cron-config.json": string(cronJSON),
	}
}

func buildAgentsMD(role string) string {
	base := "# Agent Behavior Constraints\n\nYou are an AI agent operating within the AI DevOps Platform.\nAlways follow the instructions in SOUL.md.\n"
	switch role {
	case "developer":
		base += "\n## Developer Role\nFocus on code quality, testing, and CI green state.\n"
	case "reviewer":
		base += "\n## Reviewer Role\nReview PRs thoroughly for bugs, security, and quality. Approve or request changes.\n"
	case "sre":
		base += "\n## SRE Role\nMonitor deployments, handle incidents, and maintain system reliability.\n"
	case "observer":
		base += "\n## Observer Role\nScan repositories, monitor CI, and create Gitea Issues for problems found.\n"
	}
	return base
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
	data := map[string]string{
		skillKey("gitea-api/SKILL.md"): "# Gitea API Skill\nUse the Gitea REST API to manage issues, PRs, and repositories.\n",
	}
	if agent.Spec.Role == "developer" || agent.Spec.Role == "sre" {
		data[skillKey("git-ops/SKILL.md")] = "# Git Ops Skill\nClone, branch, commit and push code changes using Git.\n"
	}
	if agent.Spec.Role == "sre" {
		data[skillKey("kubectl-ops/SKILL.md")] = "# Kubectl Ops Skill\nManage Kubernetes deployments in the app-staging namespace using kubectl.\n"
	}
	// Merge role skills (do not override builtins)
	if rd != nil {
		for k, v := range rd.Skills {
			if _, exists := data[k]; !exists {
				data[k] = v
			}
		}
	}
	return data
}

// buildSkillItems returns the ConfigMap items mapping for the skills volume,
// translating "git-ops__SKILL.md" keys back to "git-ops/SKILL.md" paths.
func buildSkillItems(agent *agentapi.Agent, rd *roleData) []corev1.KeyToPath {
	skillPaths := []string{"gitea-api/SKILL.md"}
	if agent.Spec.Role == "developer" || agent.Spec.Role == "sre" {
		skillPaths = append(skillPaths, "git-ops/SKILL.md")
	}
	if agent.Spec.Role == "sre" {
		skillPaths = append(skillPaths, "kubectl-ops/SKILL.md")
	}
	// Collect builtin keys for dedup
	builtinKeys := map[string]bool{}
	for _, p := range skillPaths {
		builtinKeys[skillKey(p)] = true
	}
	// Add role skill paths (do not override builtins)
	if rd != nil {
		for k := range rd.Skills {
			if !builtinKeys[k] {
				// Key is already in skillKey format from ConfigMap
				skillPaths = append(skillPaths, strings.ReplaceAll(k, "__", "/"))
			}
		}
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

	desired := buildDeployment(agent, r.roleData, cfgHash, r.GatewayURL)
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

func buildDeployment(agent *agentapi.Agent, rd *roleData, cfgHash string, gatewayURL string) *appsv1.Deployment {
	replicas := int32(1)
	if agent.Spec.Paused {
		replicas = 0
	}

	saName := fmt.Sprintf("agent-%s", agent.Name)
	configCMName := fmt.Sprintf("agent-%s-config", agent.Name)
	skillsCMName := fmt.Sprintf("agent-%s-skills", agent.Name)
	jwtSecretName := fmt.Sprintf("agent-%s-jwt-token", agent.Name)
	pvcName := fmt.Sprintf("agent-%s-workspace", agent.Name)

	model := ""
	if agent.Spec.LLM != nil && agent.Spec.LLM.Model != "" {
		model = agent.Spec.LLM.Model
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "config", MountPath: "/agent/config", ReadOnly: true},
		{Name: "skills", MountPath: "/agent/skills", ReadOnly: true},
		{Name: "jwt-token", MountPath: "/agent/secrets/jwt-token", SubPath: "token", ReadOnly: true},
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
							{Name: "GATEWAY_URL", Value: gatewayURL},
							{Name: "LLM_MODEL", Value: model},
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
