#!/usr/bin/env sh
set -eu

export APP_TCP_ADDR="${APP_TCP_ADDR:-127.0.0.1:9000}"
export APP_HTTP_ADDR="${APP_HTTP_ADDR:-127.0.0.1:8081}"
export APP_HTTP_TCP_ADDR="${APP_HTTP_TCP_ADDR:-127.0.0.1:9000}"
export APP_PROFILE_PICTURE_DIR="${APP_PROFILE_PICTURE_DIR:-/app/runtime/profile-pictures}"
export APP_PROFILE_PICTURE_BASE_URL="${APP_PROFILE_PICTURE_BASE_URL:-/api/me/profile-picture}"
export APP_REDIS_KEY_PREFIX="${APP_REDIS_KEY_PREFIX:-go-entry-task}"

mkdir -p "$APP_PROFILE_PICTURE_DIR" /app/runtime/logs

if [ "${RUN_MIGRATION:-false}" = "true" ]; then
  /app/bin/migrate -config /app/config/config.json -schema /app/db/schema.sql
fi

/app/bin/tcpd -config /app/config/config.json > /app/runtime/logs/tcpd.log 2>&1 &
TCPD_PID="$!"

/app/bin/httpd -config /app/config/config.json > /app/runtime/logs/httpd.log 2>&1 &
HTTPD_PID="$!"

trap 'kill "$TCPD_PID" "$HTTPD_PID" 2>/dev/null || true' INT TERM

nginx -g 'daemon off;'

