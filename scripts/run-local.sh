#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_FILE="$ROOT_DIR/configs/config.yaml"
EXAMPLE_FILE="$ROOT_DIR/configs/config.yaml.example"

if [[ ! -f "$CONFIG_FILE" ]]; then
  cp "$EXAMPLE_FILE" "$CONFIG_FILE"
  echo "Created $CONFIG_FILE from example template."
fi

: "${NOVELFORGE_LLM_API_KEY:=local-placeholder}"
export NOVELFORGE_LLM_API_KEY

if grep -q 'provider: "postgres"' "$CONFIG_FILE"; then
  : "${NOVELFORGE_DATABASE_URL:=postgres://novelforge:novelforge@127.0.0.1:5432/novelforge?sslmode=disable}"
  export NOVELFORGE_DATABASE_URL
  go -C "$ROOT_DIR" run ./cmd/migrate -config "$CONFIG_FILE"
fi

go -C "$ROOT_DIR" run ./cmd/server -config "$CONFIG_FILE"
