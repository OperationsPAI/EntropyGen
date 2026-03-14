package k8sclient

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	agentapi "github.com/entropyGen/entropyGen/internal/operator/api"
)

// workspaceMountPath is the path where agent workspace PVC is mounted inside the container.
const workspaceMountPath = "/home/node/.openclaw/workspace"

// AgentClient wraps a controller-runtime client for Agent CR operations.
type AgentClient struct {
	client     ctrlclient.Client
	k8s        kubernetes.Interface // for Pod log streaming and exec
	restConfig *rest.Config         // for Pod exec (remotecommand)
	namespace  string
}

// NewAgentClient creates an AgentClient. k8s may be nil if log retrieval is not needed.
func NewAgentClient(client ctrlclient.Client, namespace string) *AgentClient {
	return &AgentClient{client: client, namespace: namespace}
}

// NewAgentClientWithKube creates an AgentClient with a separate kubernetes clientset
// for log access and exec. restCfg may be nil if exec is not needed.
func NewAgentClientWithKube(client ctrlclient.Client, k8s kubernetes.Interface, restCfg *rest.Config, namespace string) *AgentClient {
	return &AgentClient{client: client, k8s: k8s, restConfig: restCfg, namespace: namespace}
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
	pod, err := a.findAgentPod(ctx, agentName)
	if err != nil {
		return "", err
	}
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

// ResetMemory clears the workspace PVC contents and deletes the agent Pod
// to trigger a restart via the Deployment controller.
func (a *AgentClient) ResetMemory(ctx context.Context, agentName string) error {
	if err := a.ensureClient(); err != nil {
		return err
	}
	if a.k8s == nil {
		return fmt.Errorf("k8s clientset not configured for pod operations")
	}
	if a.restConfig == nil {
		return fmt.Errorf("rest config not configured for exec operations")
	}

	// Verify agent CR exists.
	if _, err := a.Get(ctx, agentName); err != nil {
		return err
	}

	pod, err := a.findAgentPod(ctx, agentName)
	if err != nil {
		return err
	}

	// Exec rm -rf on workspace contents inside the running container.
	if err := a.execInPod(ctx, pod.Name, "agent-runtime", []string{
		"sh", "-c", "rm -rf " + workspaceMountPath + "/*",
	}); err != nil {
		return fmt.Errorf("clear workspace for agent %s: %w", agentName, err)
	}

	// Delete the pod; the Deployment controller will recreate it.
	if err := a.k8s.CoreV1().Pods(a.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete pod for agent %s: %w", agentName, err)
	}

	return nil
}

// findAgentPod returns the first pod matching the agent label selector.
func (a *AgentClient) findAgentPod(ctx context.Context, agentName string) (*corev1.Pod, error) {
	podList, err := a.k8s.CoreV1().Pods(a.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=agent-" + agentName,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods for agent %s: %w", agentName, err)
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no pods found for agent %q", agentName)
	}
	return &podList.Items[0], nil
}

// execInPod runs a command in a container via the Kubernetes exec API.
func (a *AgentClient) execInPod(ctx context.Context, podName, container string, command []string) error {
	req := a.k8s.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(a.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(a.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("create exec for pod %s: %w", podName, err)
	}

	var stdout, stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return fmt.Errorf("exec in pod %s failed (stderr: %s): %w", podName, stderr.String(), err)
	}

	return nil
}
