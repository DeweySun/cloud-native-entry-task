#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$ROOT/runtime/nginx" "$ROOT/runtime/profile-pictures"
nginx -p "$ROOT/runtime/nginx" -c "$ROOT/deploy/nginx.conf"
echo "Nginx started at http://127.0.0.1:8080"

