#!/usr/bin/env bash
set -euo pipefail

go run ./cmd/tcpd -config config/config.json

