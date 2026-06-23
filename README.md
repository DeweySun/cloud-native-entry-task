# Go Entry User Management

This project implements a two-process Go user management system:

- Nginx serves static files from `web/static`.
- Nginx exports port `8080` and proxies `/api/*` to the Go HTTP gateway.
- `cmd/httpd` accepts HTTP API requests and forwards each request to the TCP backend.
- `cmd/tcpd` owns authentication, user profile logic, profile picture storage, and MySQL access.
- Redis is optional locally; when configured, the TCP backend stores session tokens and profile picture cache entries in Redis.

## DBCP CRD Shape

The DBCP platform-facing config is represented by `deploy/dbcp-crd.yaml`.

```yaml
spec:
  targetDB: "go_entry_app:<password>@tcp(mysql.example:3306)/go_entry_user_mgmt?charset=utf8mb4&parseTime=true&loc=Local"
  targetRedis: "redis.example:6379"
  serviceExportPort: 8080
```

These fields map to `config/config.json` under `dbcp.target_db`, `dbcp.target_redis`, and `dbcp.service_export_port`. `target_db` overrides `database.dsn`; `target_redis` enables Redis-backed sessions and profile picture caching.

Profile picture reads use `/api/me/profile-picture?v=<version>`. The TCP backend first checks Redis, then falls back to the saved file and writes the file contents back to Redis. Uploading a new profile picture updates MySQL metadata and deletes the old Redis picture cache key before writing the new one.

## Local Setup

```bash
chmod +x scripts/*.sh
scripts/init-mysql.sh
go run ./cmd/migrate -config config/config.json
go run ./cmd/seed -config config/config.json -count 200
scripts/start-all.sh
```

Open `http://127.0.0.1:8080`.

The default seeded password is `Password123!`.

Stop local services with:

```bash
scripts/stop-all.sh
```
