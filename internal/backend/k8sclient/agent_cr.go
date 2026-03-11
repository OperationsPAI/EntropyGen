package k8sclient

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

// AgentClient wraps a controller-runtime client for Agent CR operations.
type AgentClient struct {
	client    ctrlclient.Client
	k8s       kubernetes.Interface // for Pod log streaming
	namespace string
}

// NewAgentClient creates an AgentClient. k8s may be nil if log retrieval is not needed.
func NewAgentClient(client ctrlclient.Client, namespace string) *AgentClient {
	return &AgentClient{client: client, namespace: namespace}
}

// NewAgentClientWithKube creates an AgentClient with a separate kubernetes clientset for log access.
func NewAgentClientWithKube(client ctrlclient.Client, k8s kubernetes.Interface, namespace string) *AgentClient {
	return &AgentClient{client: client, k8s: k8s, namespace: namespace}
}

func (a *AgentClient) ensureClient() error {
	if a.client == nil {
		return fmt.Errorf("k8s client not available")
	}
	return nil
}

// List returns all Agent CRs in the configured namespace.
func (a *AgentClient) List(ctx context.Context) ([]agentapi.Agent, error) {
	if err := a.ensureClient(); err != nil {
		return nil, err
	}
	var list agentapi.AgentList
	if err := a.client.List(ctx, &list, ctrlclient.InNamespace(a.namespace)); err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	return list.Items, nil
}

// Get returns a single Agent CR by name.
func (a *AgentClient) Get(ctx context.Context, name string) (*agentapi.Agent, error) {
	if err := a.ensureClient(); err != nil {
		return nil, err
	}
	var agent agentapi.Agent
	key := ctrlclient.ObjectKey{Namespace: a.namespace, Name: name}
	if err := a.client.Get(ctx, key, &agent); err != nil {
		return nil, fmt.Errorf("get agent %s: %w", name, err)
	}
	return &agent, nil
}

// Create creates a new Agent CR with the given name and spec.
func (a *AgentClient) Create(ctx context.Context, name string, spec agentapi.AgentSpec) (*agentapi.Agent, error) {
	if err := a.ensureClient(); err != nil {
		return nil, err
	}
	agent := &agentapi.Agent{}
	agent.SetName(name)
	agent.SetNamespace(a.namespace)
	agent.Spec = spec
	if err := a.client.Create(ctx, agent); err != nil {
		return nil, fmt.Errorf("create agent %s: %w", name, err)
	}
	return agent, nil
}

// Update patches the spec of an existing Agent CR.
func (a *AgentClient) Update(ctx context.Context, name string, spec agentapi.AgentSpec) (*agentapi.Agent, error) {
	if err := a.ensureClient(); err != nil {
		return nil, err
	}
	agent, err := a.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	agent.Spec = spec
	if err := a.client.Update(ctx, agent); err != nil {
		return nil, fmt.Errorf("update agent %s: %w", name, err)
	}
	return agent, nil
}

// Delete removes an Agent CR by name.
func (a *AgentClient) Delete(ctx context.Context, name string) error {
	if err := a.ensureClient(); err != nil {
		return err
	}
	agent, err := a.Get(ctx, name)
	if err != nil {
		return err
	}
	return a.client.Delete(ctx, agent)
}

// SetPaused sets spec.paused on an Agent CR.
func (a *AgentClient) SetPaused(ctx context.Context, name string, paused bool) (*agentapi.Agent, error) {
	if err := a.ensureClient(); err != nil {
		return nil, err
	}
	agent, err := a.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	agent.Spec.Paused = paused
	if err := a.client.Update(ctx, agent); err != nil {
		return nil, fmt.Errorf("set paused on agent %s: %w", name, err)
	}
	return agent, nil
}

// SetStatusPhase patches the status.phase field of an Agent CR.
func (a *AgentClient) SetStatusPhase(ctx context.Context, name, phase string) error {
	if err := a.ensureClient(); err != nil {
		return err
	}
	agent, err := a.Get(ctx, name)
	if err != nil {
		return err
	}
	agent.Status.Phase = phase
	return a.client.Status().Update(ctx, agent)
}

// GetLogs returns the last tailLines lines from the agent Pod's container.
func (a *AgentClient) GetLogs(ctx context.Context, agentName string, tailLines int64) (string, error) {
	if err := a.ensureClient(); err != nil {
		return "", err
	}
	if a.k8s == nil {
		return "", fmt.Errorf("k8s clientset not configured for log access")
	}
	podList, err := a.k8s.CoreV1().Pods(a.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=agent-" + agentName,
	})
	if err != nil {
		return "", fmt.Errorf("list pods for agent %s: %w", agentName, err)
	}
	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pods found for agent %q", agentName)
	}
	pod := podList.Items[0]
	req := a.k8s.CoreV1().Pods(a.namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: "agent-runtime",
		TailLines: &tailLines,
	})
	rc, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("log stream for agent %s: %w", agentName, err)
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		return "", fmt.Errorf("read logs for agent %s: %w", agentName, err)
	}
	return buf.String(), nil
}
