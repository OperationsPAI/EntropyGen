package reconciler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

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
	gatewayURL         = "http://agent-gateway.control-plane.svc:8080"
	agentRuntimeImage  = "registry.devops.local/platform/agent-runtime:latest"
	defaultStorageSize = "1Gi"
	annotationCfgHash  = "aidevops.io/config-hash"
)

// EnsureConfigMap creates or updates the main agent ConfigMap (openclaw.json, SOUL.md, AGENTS.md, cron-config.json).
func (r *ResourceReconciler) EnsureConfigMap(ctx context.Context, agent *agentapi.Agent) error {
	data := buildConfigMapData(agent)
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

func buildConfigMapData(agent *agentapi.Agent) map[string]string {
	model := "gpt-4o"
	if agent.Spec.LLM != nil && agent.Spec.LLM.Model != "" {
		model = agent.Spec.LLM.Model
	}

	openclawCfg := map[string]interface{}{
		"agent": map[string]interface{}{"model": model},
		"providers": map[string]interface{}{
			"anthropic": map[string]interface{}{
				"baseURL": gatewayURL + "/v1",
				"apiKey":  "__JWT_PLACEHOLDER__",
			},
		},
		"automation": map[string]interface{}{
			"webhook": map[string]interface{}{"enabled": true, "port": 9090},
		},
	}
	openclawJSON, _ := json.MarshalIndent(openclawCfg, "", "  ")

	cronCfg := map[string]interface{}{}
	if agent.Spec.Cron != nil {
		cronCfg["schedule"] = agent.Spec.Cron.Schedule
		cronCfg["prompt"] = agent.Spec.Cron.Prompt
	}
	cronJSON, _ := json.MarshalIndent(cronCfg, "", "  ")

	return map[string]string{
		"openclaw.json":    string(openclawJSON),
		"SOUL.md":          agent.Spec.Soul,
		"AGENTS.md":        buildAgentsMD(agent.Spec.Role),
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
	data := buildSkillsData(agent)
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

func buildSkillsData(agent *agentapi.Agent) map[string]string {
	data := map[string]string{
		"gitea-api/SKILL.md": "# Gitea API Skill\nUse the Gitea REST API to manage issues, PRs, and repositories.\n",
	}
	if agent.Spec.Role == "developer" || agent.Spec.Role == "sre" {
		data["git-ops/SKILL.md"] = "# Git Ops Skill\nClone, branch, commit and push code changes using Git.\n"
	}
	if agent.Spec.Role == "sre" {
		data["kubectl-ops/SKILL.md"] = "# Kubectl Ops Skill\nManage Kubernetes deployments in the app-staging namespace using kubectl.\n"
	}
	return data
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

// EnsureRoleBinding creates the RoleBinding for the agent.
// SRE agents get an additional RoleBinding in app-staging namespace.
func (r *ResourceReconciler) EnsureRoleBinding(ctx context.Context, agent *agentapi.Agent) error {
	saName := fmt.Sprintf("agent-%s", agent.Name)
	rbName := fmt.Sprintf("agent-%s", agent.Name)

	rb := &rbacv1.RoleBinding{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: rbName, Namespace: agent.Namespace}, rb)
	if errors.IsNotFound(err) {
		newRB := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: agent.Namespace},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "agent-readonly-role",
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

	// SRE: extra RoleBinding in app-staging
	if agent.Spec.Role == "sre" {
		sreRBName := fmt.Sprintf("agent-%s-app-staging", agent.Name)
		sreRB := &rbacv1.RoleBinding{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: sreRBName, Namespace: "app-staging"}, sreRB)
		if errors.IsNotFound(err) {
			newSreRB := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: sreRBName, Namespace: "app-staging"},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     "sre-agent-role",
				},
				Subjects: []rbacv1.Subject{{
					Kind:      "ServiceAccount",
					Name:      saName,
					Namespace: agent.Namespace,
				}},
			}
			return r.Client.Create(ctx, newSreRB)
		}
		return err
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

	desired := buildDeployment(agent, cfgHash)
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
	if len(existing.Spec.Template.Spec.Containers) > 0 {
		existing.Spec.Template.Spec.Containers[0].Resources = desired.Spec.Template.Spec.Containers[0].Resources
	}
	if existing.Spec.Template.Annotations == nil {
		existing.Spec.Template.Annotations = map[string]string{}
	}
	existing.Spec.Template.Annotations[annotationCfgHash] = cfgHash
	return r.Client.Update(ctx, existing)
}

func buildDeployment(agent *agentapi.Agent, cfgHash string) *appsv1.Deployment {
	replicas := int32(1)
	if agent.Spec.Paused {
		replicas = 0
	}

	saName := fmt.Sprintf("agent-%s", agent.Name)
	configCMName := fmt.Sprintf("agent-%s-config", agent.Name)
	skillsCMName := fmt.Sprintf("agent-%s-skills", agent.Name)
	jwtSecretName := fmt.Sprintf("agent-%s-jwt-token", agent.Name)
	pvcName := fmt.Sprintf("agent-%s-workspace", agent.Name)

	model := "gpt-4o"
	if agent.Spec.LLM != nil && agent.Spec.LLM.Model != "" {
		model = agent.Spec.LLM.Model
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
						Image:     agentRuntimeImage,
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
						VolumeMounts: []corev1.VolumeMount{
							{Name: "config", MountPath: "/home/node/.openclaw"},
							{Name: "skills", MountPath: "/home/node/.openclaw/skills"},
							{Name: "jwt-token", MountPath: "/agent/secrets/jwt-token", SubPath: "token"},
							{Name: "workspace", MountPath: "/home/node/.openclaw/workspace"},
						},
					}},
					Volumes: []corev1.Volume{
						{Name: "config", VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: configCMName},
							},
						}},
						{Name: "skills", VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: skillsCMName},
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
					},
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
