package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion is group version used to register these objects
var GroupVersion = schema.GroupVersion{Group: "aidevops.io", Version: "v1alpha1"}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Role",type="string",JSONPath=".spec.role"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.llm.model"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentSpec   `json:"spec,omitempty"`
	Status AgentStatus `json:"status,omitempty"`
}

type AgentSpec struct {
	Role         string           `json:"role"`
	DisplayName  string           `json:"displayName,omitempty"`
	Cron         *AgentCron       `json:"cron,omitempty"`
	LLM          *AgentLLM        `json:"llm,omitempty"`
	Gitea        *AgentGitea      `json:"gitea,omitempty"`
	Kubernetes   *AgentKubernetes `json:"kubernetes,omitempty"`
	Resources    *AgentResources  `json:"resources,omitempty"`
	Memory       *AgentMemory     `json:"memory,omitempty"`
	RuntimeImage string           `json:"runtimeImage,omitempty"`
	Paused       bool             `json:"paused,omitempty"`
}

type AgentCron struct {
	Schedule string `json:"schedule,omitempty"`
	// Prompt field removed — cron prompt is now defined entirely by the Role's PROMPT.md
}

type AgentLLM struct {
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"maxTokens,omitempty"`
}

type AgentGitea struct {
	Username    string   `json:"username,omitempty"`
	Email       string   `json:"email,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Repos       []string `json:"repos,omitempty"`
}

type AgentKubernetes struct {
	NamespaceAccess []string `json:"namespaceAccess,omitempty"`
	RBACRole        string   `json:"rbacRole,omitempty"`
}

type ResourceRequirements struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type AgentResources struct {
	Requests *ResourceRequirements `json:"requests,omitempty"`
	Limits   *ResourceRequirements `json:"limits,omitempty"`
}

type AgentMemory struct {
	StorageSize  string `json:"storageSize,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
}

type CurrentTask struct {
	Type   string `json:"type,omitempty"`
	Number int    `json:"number,omitempty"`
	Title  string `json:"title,omitempty"`
	Repo   string `json:"repo,omitempty"`
}

type AgentStatus struct {
	Phase       string           `json:"phase,omitempty"`
	Conditions  []AgentCondition `json:"conditions,omitempty"`
	GiteaUser   *GiteaUserStatus `json:"giteaUser,omitempty"`
	LastAction  *AgentLastAction `json:"lastAction,omitempty"`
	TokenUsage  *AgentTokenUsage `json:"tokenUsage,omitempty"`
	CurrentTask *CurrentTask     `json:"currentTask,omitempty"`
	PodName     string           `json:"podName,omitempty"`
	StartedAt   *metav1.Time     `json:"startedAt,omitempty"`
}

type AgentCondition struct {
	Type               string      `json:"type"`
	Status             string      `json:"status"`
	Reason             string      `json:"reason,omitempty"`
	Message            string      `json:"message,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

type GiteaUserStatus struct {
	Created        bool   `json:"created,omitempty"`
	Username       string `json:"username,omitempty"`
	TokenSecretRef string `json:"tokenSecretRef,omitempty"`
}

type AgentLastAction struct {
	Description string      `json:"description,omitempty"`
	Timestamp   metav1.Time `json:"timestamp,omitempty"`
}

type AgentTokenUsage struct {
	Today int64 `json:"today"`
	Total int64 `json:"total"`
}

// +kubebuilder:object:root=true
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}

// DeepCopyObject implements runtime.Object
func (a *Agent) DeepCopyObject() runtime.Object {
	if c := a.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a deep copy of Agent
func (a *Agent) DeepCopy() *Agent {
	if a == nil {
		return nil
	}
	out := new(Agent)
	a.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties into another Agent instance
func (a *Agent) DeepCopyInto(out *Agent) {
	*out = *a
	out.TypeMeta = a.TypeMeta
	a.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	a.Spec.DeepCopyInto(&out.Spec)
	a.Status.DeepCopyInto(&out.Status)
}

// DeepCopyObject implements runtime.Object for AgentList
func (al *AgentList) DeepCopyObject() runtime.Object {
	if c := al.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a deep copy of AgentList
func (al *AgentList) DeepCopy() *AgentList {
	if al == nil {
		return nil
	}
	out := new(AgentList)
	al.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties into another AgentList instance
func (al *AgentList) DeepCopyInto(out *AgentList) {
	*out = *al
	out.TypeMeta = al.TypeMeta
	al.ListMeta.DeepCopyInto(&out.ListMeta)
	if al.Items != nil {
		in, o := &al.Items, &out.Items
		*o = make([]Agent, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*o)[i])
		}
	}
}

// Ensure runtime.Object interface compliance
var (
	_ runtime.Object = &Agent{}
	_ runtime.Object = &AgentList{}
)
