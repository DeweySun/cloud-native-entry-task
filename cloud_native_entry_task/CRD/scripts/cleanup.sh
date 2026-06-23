#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRD_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

kubectl delete -f "$CRD_DIR/verify-svc-job.yaml" --ignore-not-found
kubectl delete -f "$CRD_DIR/service.yaml" --ignore-not-found
kubectl delete -f "$CRD_DIR/deployment.yaml" --ignore-not-found
kubectl delete -f "$CRD_DIR/app-secret.example.yaml" --ignore-not-found
kubectl delete -f "$CRD_DIR/app-configmap.yaml" --ignore-not-found
kubectl delete -f "$CRD_DIR/dbcp-entry-service-sample.yaml" --ignore-not-found
kubectl delete -f "$CRD_DIR/dbcp-entry-service-crd.yaml" --ignore-not-found

echo "Deleted DBCP entry service resources."

