#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRD_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ROOT_DIR="$(cd "$CRD_DIR/../.." && pwd)"
IMAGE="${IMAGE:-dbcp-entry-service:local}"

cd "$ROOT_DIR"

if command -v docker >/dev/null 2>&1; then
  docker build -f "$CRD_DIR/Dockerfile" -t "$IMAGE" .
elif command -v colima >/dev/null 2>&1; then
  colima nerdctl -- --namespace k8s.io build -f "$CRD_DIR/Dockerfile" -t "$IMAGE" .
else
  echo "No supported image builder found. Install Docker CLI or use Colima with containerd/nerdctl." >&2
  exit 1
fi

echo "Built image: $IMAGE"
