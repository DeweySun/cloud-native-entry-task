#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

mkdir -p bin runtime runtime/nginx runtime/profile-pictures

GOCACHE="${GOCACHE:-$ROOT/.cache/go-build}" GOPATH="${GOPATH:-$ROOT/.gopath}" go build -o bin/tcpd ./cmd/tcpd
GOCACHE="${GOCACHE:-$ROOT/.cache/go-build}" GOPATH="${GOPATH:-$ROOT/.gopath}" go build -o bin/httpd ./cmd/httpd

if [ -f runtime/tcpd.pid ] && kill -0 "$(cat runtime/tcpd.pid)" >/dev/null 2>&1; then
  echo "TCP backend is already running."
else
  nohup bin/tcpd -config config/config.json > runtime/tcpd.log 2>&1 &
  echo $! > runtime/tcpd.pid
  echo "TCP backend started."
fi

if [ -f runtime/httpd.pid ] && kill -0 "$(cat runtime/httpd.pid)" >/dev/null 2>&1; then
  echo "HTTP gateway is already running."
else
  nohup bin/httpd -config config/config.json > runtime/httpd.log 2>&1 &
  echo $! > runtime/httpd.pid
  echo "HTTP gateway started."
fi

if [ -f runtime/nginx/nginx.pid ] && kill -0 "$(cat runtime/nginx/nginx.pid)" >/dev/null 2>&1; then
  echo "Nginx is already running."
else
  nginx -p "$ROOT/runtime/nginx" -c "$ROOT/deploy/nginx.conf"
  echo "Nginx started."
fi

echo "Static site: http://127.0.0.1:8080"
echo "HTTP API:    http://127.0.0.1:8081"

