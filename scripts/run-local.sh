#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_FILE="$ROOT_DIR/configs/config.yaml"
EXAMPLE_FILE="$ROOT_DIR/configs/config.yaml.example"
ENV_FILE="$ROOT_DIR/.env"

if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  set -a
  source "$ENV_FILE"
  set +a
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
  cp "$EXAMPLE_FILE" "$CONFIG_FILE"
  echo "Created $CONFIG_FILE from example template."
fi

: "${NOVELFORGE_LLM_API_KEY:=local-placeholder}"
export NOVELFORGE_LLM_API_KEY
: "${NOVELFORGE_LLM_PROVIDER:=openai_compatible}"
export NOVELFORGE_LLM_PROVIDER
: "${NOVELFORGE_LLM_MODEL:=gpt-4o-mini}"
export NOVELFORGE_LLM_MODEL
: "${NOVELFORGE_LLM_BASE_URL:=https://api.openai.com/v1}"
export NOVELFORGE_LLM_BASE_URL

if grep -q 'provider: "postgres"' "$CONFIG_FILE"; then
  : "${NOVELFORGE_DATABASE_URL:=postgres://novelforge:novelforge@127.0.0.1:5432/novelforge?sslmode=disable}"
  export NOVELFORGE_DATABASE_URL
  go -C "$ROOT_DIR" run ./cmd/migrate -config "$CONFIG_FILE"
fi

go -C "$ROOT_DIR" run ./cmd/server -config "$CONFIG_FILE"
