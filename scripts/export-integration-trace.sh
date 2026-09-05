#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <order-or-gateway-number> [output.jsonl]" >&2
  exit 2
fi

needle=$1
output=${2:-"output/integration-traces/${needle}.jsonl"}
mkdir -p "$(dirname "$output")"

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

ssh laoshirenvip-agent \
  "cd /opt/laoshirenvip-agent-shop/deploy && docker compose exec -T app cat /app/logs/app.log" \
  >"$tmp"

jq -c --arg needle "$needle" '
  select(
    (.message // "")
    | test("^(integration_trace_|payment_callback_|epay_callback_|procurement_)")
  )
  | select(tojson | contains($needle))
' "$tmp" >"$output"

count=$(wc -l <"$output" | tr -d ' ')
echo "exported ${count} trace records to ${output}"
