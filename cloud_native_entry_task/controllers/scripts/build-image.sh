#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTROLLER_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ROOT_DIR="$(cd "$CONTROLLER_DIR/../.." && pwd)"
IMAGE="${IMAGE:-dbcp-entry-controller:local}"

cd "$ROOT_DIR"

mkdir -p "$CONTROLLER_DIR/bin"
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOCACHE="${GOCACHE:-$ROOT_DIR/.gocache}" \
  go build -o "$CONTROLLER_DIR/bin/dbcp-controller" ./cloud_native_entry_task/controllers/cmd/dbcp-controller

if command -v docker >/dev/null 2>&1; then
  docker build -f "$CONTROLLER_DIR/Dockerfile.runtime" -t "$IMAGE" .
elif command -v colima >/dev/null 2>&1; then
  colima nerdctl -- --namespace k8s.io build -f "$CONTROLLER_DIR/Dockerfile.runtime" -t "$IMAGE" .
else
  echo "No supported image builder found. Install Docker CLI or use Colima with containerd/nerdctl." >&2
  exit 1
fi

echo "Built image: $IMAGE"
