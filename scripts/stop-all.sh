#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [ -f runtime/httpd.pid ] && kill -0 "$(cat runtime/httpd.pid)" >/dev/null 2>&1; then
  kill "$(cat runtime/httpd.pid)"
  echo "HTTP gateway stopped."
fi

if [ -f runtime/tcpd.pid ] && kill -0 "$(cat runtime/tcpd.pid)" >/dev/null 2>&1; then
  kill "$(cat runtime/tcpd.pid)"
  echo "TCP backend stopped."
fi

if [ -f runtime/nginx/nginx.pid ] && kill -0 "$(cat runtime/nginx/nginx.pid)" >/dev/null 2>&1; then
  nginx -p "$ROOT/runtime/nginx" -c "$ROOT/deploy/nginx.conf" -s quit
  echo "Nginx stopped."
fi

