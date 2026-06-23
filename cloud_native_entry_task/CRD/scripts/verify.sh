#!/usr/bin/env bash
set -euo pipefail

kubectl rollout status deployment/dbcp-entry-service --timeout=120s
kubectl get svc dbcp-entry-service
kubectl wait --for=condition=complete job/dbcp-entry-service-svc-verify --timeout=120s
kubectl logs job/dbcp-entry-service-svc-verify

