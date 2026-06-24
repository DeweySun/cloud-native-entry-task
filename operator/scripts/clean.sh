#!/bin/bash
set -euo pipefail

APP_NS="entry-system"
OPERATOR_NS="operator-system"
CR_NAME="dbcp-entry-service"
CR_KIND="dbcpentryservice"
CRD_NAME="dbcpentryservices.dbcp.shopee.io"

echo ">>> 清理业务 CR 及其管理的资源..."
# 删除 CR（如果存在），这会触发 finalizer 自动清理 Pod/Service/ConfigMap
kubectl delete "$CR_KIND" "$CR_NAME" -n "$APP_NS" --ignore-not-found

echo ">>> 卸载 Operator（包含 operator-system 命名空间、RBAC 等）..."
# 在 operator/ 目录下执行 make undeploy（需 Makefile 支持）
# 若因某些原因 make undeploy 不可用，会继续尝试手动清理
make undeploy || true

echo ">>> 删除 entry-system 命名空间（如存在）..."
kubectl delete namespace "$APP_NS" --ignore-not-found

echo ">>> 验证 CRD 是否已移除..."
if kubectl get crd "$CRD_NAME" &>/dev/null; then
  echo "WARNING: CRD 仍然存在，手动删除..."
  kubectl delete crd "$CRD_NAME"
else
  echo "CRD 已清理。"
fi

echo ">>> 清理完成。"