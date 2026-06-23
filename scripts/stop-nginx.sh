#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
nginx -p "$ROOT/runtime/nginx" -c "$ROOT/deploy/nginx.conf" -s quit

