#!/usr/bin/env bash
set -euo pipefail

CREDS="${1:-/opt/sub2api/kiro-rs/config/credentials.json}"
PROBE="${KIRO_PROBE:-/tmp/kiro_server_probe.py}"

run_probe() {
  local label="$1"
  shift
  echo "--- ${label}"
  python3 "${PROBE}" --creds "${CREDS}" "$@" | grep -E 'list_models|generate'
}

for style in kam-ide ide; do
  for origin in AI_EDITOR KIRO_IDE SM_AI_STUDIO_IDE KIRO_CLI CLI AmazonQ; do
    run_probe \
      "style=${style} origin=${origin} region=us-east-1 profile=yes" \
      --client-style "${style}" \
      --payload-style kam \
      --agent-mode vibe \
      --list-origin "${origin}" \
      --generate-origin "${origin}" \
      --model qwen3-coder-next \
      --model claude-opus-4.7 \
      --model claude-sonnet-4.6
  done

  run_probe \
    "style=${style} origin=AI_EDITOR region=us-east-1 profile=no" \
    --client-style "${style}" \
    --payload-style kam \
    --agent-mode vibe \
    --omit-profile-arn \
    --list-origin AI_EDITOR \
    --generate-origin AI_EDITOR \
    --model qwen3-coder-next \
    --model claude-opus-4.7
done

for region in eu-central-1 us-west-2; do
  run_probe \
    "style=kam-ide origin=AI_EDITOR region=${region} profile=yes" \
    --region "${region}" \
    --client-style kam-ide \
    --payload-style kam \
    --agent-mode vibe \
    --list-origin AI_EDITOR \
    --generate-origin AI_EDITOR \
    --model qwen3-coder-next \
    --model claude-opus-4.7
done
