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

go -C "$ROOT_DIR" run ./cmd/server -config "$CONFIG_FILE"
