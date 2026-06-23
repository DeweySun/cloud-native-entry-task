# DBCP Entry Service CRD

This folder contains the cloud-native deliverables for the Go entry user management service.

## Files

- `dbcp-entry-service-crd.yaml`: defines the `DbcpEntryService` CRD.
- `dbcp-entry-service-sample.yaml`: sample custom resource with target DB, target Redis, image, resources, and replicas.
- `deployment.yaml`: runs the single application image with `tcpd`, `httpd`, and Nginx.
- `service.yaml`: exposes the login UI on port `8080` inside the cluster.
- `verify-svc-job.yaml`: verifies `dbcp-entry-service.default.svc.cluster.local:8080` from inside a pod.
- `Dockerfile`: builds the single service image.
- `docker/entrypoint.sh`: starts `tcpd`, `httpd`, and Nginx in one container.

## Local Usage

Build the image:

```bash
cloud_native_entry_task/CRD/scripts/build-image.sh
```

When Colima is used with the `containerd` runtime, the script builds the image in the `k8s.io` namespace so local Kubernetes pods can use `dbcp-entry-service:local` directly.

Apply resources:

```bash
cloud_native_entry_task/CRD/scripts/apply.sh
```

Verify service DNS from inside the cluster:

```bash
cloud_native_entry_task/CRD/scripts/verify.sh
```

Cleanup:

```bash
cloud_native_entry_task/CRD/scripts/cleanup.sh
```

The sample secret points to `host.docker.internal` for MySQL and Redis. Adjust `app-secret.example.yaml` if your Colima networking setup uses a different host address.
