#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -f docker-compose.common.yml -f docker-compose.proxy.yml)

"${compose[@]}" up -d --build postgres redis metergate sample-api sample-web
"${compose[@]}" exec -T metergate npm run prisma:migrate
"${compose[@]}" exec -T metergate npm run prisma:seed

curl -fsS http://localhost:3000/health >/dev/null
curl -fsS http://localhost:3000/docs-json >/dev/null
curl -fsS http://localhost:8000/health >/dev/null

curl -fsS \
  -H "content-type: application/json" \
  -H "x-api-key: mg_demo_local_plaintext_key" \
  -d '{"query":"query { tickets { id subject status } }"}' \
  http://localhost:3000/graphql >/dev/null

blocked=0
for _ in $(seq 1 10); do
  status=$(curl -sS -o /tmp/metergate-proxy-response.json -w "%{http_code}" \
    -H "content-type: application/json" \
    -H "x-api-key: mg_demo_local_plaintext_key" \
    -d '{"query":"query { tickets { id } }"}' \
    http://localhost:3000/graphql)
  if [[ "$status" == "429" ]]; then
    blocked=1
    break
  fi
done

test "$blocked" = "1"
echo "proxy smoke passed"

