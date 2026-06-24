#!/bin/bash
set -euo pipefail

# ---- 配置 ----
CLUSTER_NAME="kind"
OPERATOR_NS="operator-system"
APP_NS="entry-system"
CR_SAMPLE="../cloud_native_entry_task/CRD/dbcp-entry-service-sample.yaml"   # CR 示例文件路径
SERVICE_NAME="dbcp-entry-service"
LOCAL_PORT=18080
REMOTE_PORT=8080

# ---- 函数：打印步骤 ----
step() {
  echo "=============================================="
  echo ">>> $1"
  echo "=============================================="
}

# ---- 1. 创建业务命名空间（若不存在）----
step "Creating namespace $APP_NS if not exists..."
kubectl create namespace "$APP_NS" --dry-run=client -o yaml | kubectl apply -f -

# ---- 2. 加载镜像到 kind 集群 ----
step "Loading docker images into kind cluster..."
kind load docker-image dbcp-entry-operator:local --name "$CLUSTER_NAME"
kind load docker-image dbcp-entry-service:local --name "$CLUSTER_NAME"

# ---- 3. 安装 CRD 并部署 Operator ----
step "Deploying Operator (CRD + Deployment)..."
make deploy

# ---- 4. 等待 Operator Pod 就绪 ----
step "Waiting for Operator deployment to be ready..."
kubectl -n "$OPERATOR_NS" wait --for=condition=Available deployment/operator-controller-manager --timeout=120s

# ---- 5. 创建业务 CR ----
step "Creating DbcpEntryService custom resource..."
if [ ! -f "$CR_SAMPLE" ]; then
  echo "ERROR: CR sample file not found at $CR_SAMPLE"
  exit 1
fi
kubectl apply -f "$CR_SAMPLE"

# ---- 6. 等待业务 Service 创建 ----
step "Waiting for entry service Service to be created..."
until kubectl -n "$APP_NS" get service "$SERVICE_NAME" &>/dev/null; do
  sleep 2
done

# ---- 7. 等待业务 Pod 就绪（至少一个副本） ----
step "Waiting for entry service Pod to be ready..."
kubectl -n "$APP_NS" wait --for=condition=Ready pod -l app.kubernetes.io/instance="$SERVICE_NAME" --timeout=120s

# ---- 8. 端口转发 ----
step "Starting port forwarding: localhost:$LOCAL_PORT -> $SERVICE_NAME:$REMOTE_PORT"
echo "Access the application at http://localhost:$LOCAL_PORT"
echo "Press Ctrl+C to stop."
kubectl -n "$APP_NS" port-forward "service/$SERVICE_NAME" "$LOCAL_PORT:$REMOTE_PORT"