#!/usr/bin/env bash
set -euo pipefail

echo "[+] Building fetch-relay"
go build -o fetch-relay ./cmd/fetch-relay

echo "[+] Running data ingestion"
./fetch-relay
