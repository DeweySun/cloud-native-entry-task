# DBCP Entry Service Controller

This controller reconciles `DbcpEntryService` custom resources and directly manages the service pods, service, and configmap.

## Behavior

- Adds a finalizer to each `DbcpEntryService` CR.
- Creates or updates `<cr-name>-config` from `spec.config.targetDB`, `spec.config.targetRedis`, and `spec.config.serviceExportPort`.
- Creates or updates a ClusterIP Service named `<cr-name>`.
- Creates application Pods directly from `spec.service.image`, `spec.service.resources`, and `spec.service.replicas`.
- Deletes extra Pods when replicas decrease.
- Creates replacement Pods when replicas increase or a managed Pod is deleted.
- Deletes Pods with an old spec hash when image, config, service port, or resources change.
- Deletes managed Pods, Service, and ConfigMap before removing the CR finalizer.

## Local Usage

Build the controller image:

```bash
cloud_native_entry_task/controllers/scripts/build-image.sh
```

The script cross-compiles the controller locally and builds a small runtime image with `Dockerfile.runtime`. The multi-stage `Dockerfile` is kept for CI or remote builders.

Apply controller RBAC and Deployment:

```bash
cloud_native_entry_task/controllers/scripts/apply.sh
```

Verify the controller:

```bash
cloud_native_entry_task/controllers/scripts/verify.sh
```

Cleanup controller resources:

```bash
cloud_native_entry_task/controllers/scripts/cleanup.sh
```

Deploy CRD:

```bash
kubectl apply -f cloud_native_entry_task/CRD/dbcp-entry-service-sample.yaml
```

Observe auto-created resources:

```bash
kubectl get pods,svc,cm -l app.kubernetes.io/managed-by=dbcp-entry-controller
```

Delete CR:

```bash
kubectl delete dbcp-entry-services.dbcp.shopee.io dbcp-entry-service
```

Expose the service port:

```bash
kubectl port-forward svc/dbcp-entry-service 18080:8080
```


The controller image build script writes to the Colima `k8s.io` containerd namespace when Docker CLI is unavailable, so `dbcp-entry-controller:local` can be used by local Kubernetes pods directly.
