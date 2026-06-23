#!/usr/bin/env bash
set -euo pipefail

kubectl rollout status deployment/dbcp-entry-controller --timeout=120s
kubectl get deploy/dbcp-entry-controller
kubectl logs deployment/dbcp-entry-controller --tail=80
