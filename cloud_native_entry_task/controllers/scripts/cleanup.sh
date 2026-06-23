#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTROLLER_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

kubectl delete -f "$CONTROLLER_DIR/deploy/controller-deployment.yaml" --ignore-not-found
kubectl delete -f "$CONTROLLER_DIR/deploy/rbac.yaml" --ignore-not-found

echo "Deleted DBCP entry service controller resources."
