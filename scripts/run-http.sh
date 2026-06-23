#!/usr/bin/env bash
set -euo pipefail

go run ./cmd/httpd -config config/config.json

