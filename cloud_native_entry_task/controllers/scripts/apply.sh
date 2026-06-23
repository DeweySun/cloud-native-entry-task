#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTROLLER_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ROOT_DIR="$(cd "$CONTROLLER_DIR/../.." && pwd)"

kubectl apply -f "$ROOT_DIR/cloud_native_entry_task/CRD/dbcp-entry-service-crd.yaml"
kubectl apply -f "$CONTROLLER_DIR/deploy/rbac.yaml"
kubectl apply -f "$CONTROLLER_DIR/deploy/controller-deployment.yaml"

echo "Applied DBCP entry service controller resources."
