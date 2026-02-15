#!/usr/bin/env bash
set -euo pipefail

echo "[+] Building threshold-analysis"
go build -o threshold-analysis ./cmd/threshold-analysis

echo "[+] Running threshold discovery analysis"
./threshold-analysis
