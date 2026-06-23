#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRD_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

kubectl apply -f "$CRD_DIR/dbcp-entry-service-crd.yaml"
kubectl apply -f "$CRD_DIR/dbcp-entry-service-sample.yaml"
kubectl apply -f "$CRD_DIR/app-configmap.yaml"

if [ "${USE_EXAMPLE_SECRET:-true}" = "true" ]; then
  kubectl apply -f "$CRD_DIR/app-secret.example.yaml"
else
  echo "Skipping example secret. Create dbcp-entry-service-secret before applying Deployment." >&2
fi

kubectl apply -f "$CRD_DIR/deployment.yaml"
kubectl apply -f "$CRD_DIR/service.yaml"
kubectl delete job dbcp-entry-service-svc-verify --ignore-not-found
kubectl apply -f "$CRD_DIR/verify-svc-job.yaml"

echo "Applied DBCP entry service resources."

