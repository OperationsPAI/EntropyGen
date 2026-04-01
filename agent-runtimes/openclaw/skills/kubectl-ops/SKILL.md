# Kubernetes Operations Skill

## Important

This skill uses in-cluster ServiceAccount credentials. Never specify `--kubeconfig`.

## Namespace Restriction

ALL operations MUST use `-n app-staging`. Never operate in other namespaces.

## Viewing Resources

### Pods

```bash
# List all pods
kubectl get pods -n app-staging

# Filter pods by label
kubectl get pods -n app-staging -l app=api-server

# Describe a specific pod
kubectl describe pod <pod-name> -n app-staging

# View pod logs (last 100 lines)
kubectl logs <pod-name> -n app-staging --tail=100

# Follow pod logs in real time
kubectl logs <pod-name> -n app-staging -f
```

### Deployments

```bash
# List deployments
kubectl get deployments -n app-staging

# Check rollout status
kubectl rollout status deployment/<name> -n app-staging

# View rollout history
kubectl rollout history deployment/<name> -n app-staging

# Rollback to previous version
kubectl rollout undo deployment/<name> -n app-staging
```

## Applying Manifests

```bash
kubectl apply -f manifest.yaml -n app-staging
```

## Checking Events

```bash
# List events sorted by timestamp
kubectl get events -n app-staging --sort-by='.lastTimestamp'
```

## Services and Endpoints

```bash
# List services
kubectl get svc -n app-staging

# View endpoints
kubectl get endpoints -n app-staging
```

## Forbidden Operations

The following operations are never permitted:

- Operating in `kube-system`, `control-plane`, or `gitea` namespaces
- `kubectl delete` without explicit user confirmation
- `kubectl exec` into pods
- Modifying RBAC resources (roles, rolebindings, clusterroles, clusterrolebindings)
- `kubectl edit` on any resource
- `kubectl scale` without explicit user confirmation
