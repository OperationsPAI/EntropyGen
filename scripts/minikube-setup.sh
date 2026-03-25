#!/usr/bin/env bash
# minikube-setup.sh — Install and configure minikube + skaffold for local development.
# Usage: ./scripts/minikube-setup.sh [start|stop|delete|status]
set -euo pipefail

MINIKUBE_PROFILE="aidevops"
MINIKUBE_CPUS="${MINIKUBE_CPUS:-4}"
MINIKUBE_MEMORY="${MINIKUBE_MEMORY:-8192}"
MINIKUBE_DISK="${MINIKUBE_DISK:-40g}"
MINIKUBE_K8S_VERSION="${MINIKUBE_K8S_VERSION:-v1.31.0}"

LOCAL_BIN="${HOME}/.local/bin"
mkdir -p "$LOCAL_BIN"
export PATH="${LOCAL_BIN}:${PATH}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# ── Install dependencies ────────────────────────────────────────────
install_minikube() {
  if command -v minikube &>/dev/null; then
    info "minikube already installed: $(minikube version --short)"
    return
  fi
  info "Installing minikube..."
  curl -Lo /tmp/minikube https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
  install -m 0755 /tmp/minikube "${LOCAL_BIN}/minikube"
  rm -f /tmp/minikube
  info "minikube installed: $(minikube version --short)"
}

install_skaffold() {
  if command -v skaffold &>/dev/null; then
    info "skaffold already installed: $(skaffold version)"
    return
  fi
  info "Installing skaffold..."
  curl -Lo /tmp/skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64
  install -m 0755 /tmp/skaffold "${LOCAL_BIN}/skaffold"
  rm -f /tmp/skaffold
  info "skaffold installed: $(skaffold version)"
}

install_helm() {
  if command -v helm &>/dev/null; then
    info "helm already installed: $(helm version --short)"
    return
  fi
  info "Installing helm..."
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | USE_SUDO=false HELM_INSTALL_DIR="${LOCAL_BIN}" bash
  info "helm installed: $(helm version --short)"
}

install_kubectl() {
  if command -v kubectl &>/dev/null; then
    info "kubectl already installed: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"
    return
  fi
  info "Installing kubectl..."
  curl -Lo /tmp/kubectl "https://dl.k8s.io/release/$(curl -sL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
  install -m 0755 /tmp/kubectl "${LOCAL_BIN}/kubectl"
  rm -f /tmp/kubectl
  info "kubectl installed"
}

check_docker() {
  if ! command -v docker &>/dev/null; then
    error "Docker is required but not installed. Please install Docker first."
    exit 1
  fi
  if ! docker info &>/dev/null; then
    error "Docker daemon is not running or current user lacks permissions."
    error "Try: sudo systemctl start docker && sudo usermod -aG docker \$USER"
    exit 1
  fi
  info "Docker is available"
}

# ── Cluster lifecycle ───────────────────────────────────────────────
start_cluster() {
  check_docker
  install_minikube
  install_kubectl
  install_helm
  install_skaffold

  if minikube status -p "$MINIKUBE_PROFILE" &>/dev/null; then
    info "minikube profile '$MINIKUBE_PROFILE' is already running"
  else
    info "Starting minikube (profile=$MINIKUBE_PROFILE, cpus=$MINIKUBE_CPUS, memory=$MINIKUBE_MEMORY, disk=$MINIKUBE_DISK)..."
    minikube start \
      -p "$MINIKUBE_PROFILE" \
      --driver=docker \
      --cpus="$MINIKUBE_CPUS" \
      --memory="$MINIKUBE_MEMORY" \
      --disk-size="$MINIKUBE_DISK" \
      --kubernetes-version="$MINIKUBE_K8S_VERSION" \
      --addons=default-storageclass,storage-provisioner,metrics-server
  fi

  info "Setting kubectl context to minikube profile '$MINIKUBE_PROFILE'..."
  minikube profile "$MINIKUBE_PROFILE"

  info ""
  info "=========================================="
  info " minikube is ready!"
  info "=========================================="
  info ""
  info "Deploy with skaffold:"
  info "  skaffold run -p minikube"
  info ""
  info "Dev mode (hot-reload):"
  info "  skaffold dev -p minikube"
  info ""
  info "Access frontend:"
  info "  minikube service -p $MINIKUBE_PROFILE -n aidevops control-panel-frontend"
  info ""
  info "Access Gitea:"
  info "  minikube service -p $MINIKUBE_PROFILE -n aidevops gitea"
  info ""
  info "Point shell to minikube docker daemon:"
  info "  eval \$(minikube -p $MINIKUBE_PROFILE docker-env)"
  info ""
}

stop_cluster() {
  info "Stopping minikube profile '$MINIKUBE_PROFILE'..."
  minikube stop -p "$MINIKUBE_PROFILE"
  info "Stopped."
}

delete_cluster() {
  info "Deleting minikube profile '$MINIKUBE_PROFILE'..."
  minikube delete -p "$MINIKUBE_PROFILE"
  info "Deleted."
}

show_status() {
  minikube status -p "$MINIKUBE_PROFILE" 2>/dev/null || warn "Profile '$MINIKUBE_PROFILE' does not exist yet. Run: $0 start"
}

# ── Main ────────────────────────────────────────────────────────────
ACTION="${1:-start}"
case "$ACTION" in
  start)  start_cluster ;;
  stop)   stop_cluster  ;;
  delete) delete_cluster ;;
  status) show_status   ;;
  *)
    echo "Usage: $0 [start|stop|delete|status]"
    exit 1
    ;;
esac
