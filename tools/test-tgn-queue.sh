#!/usr/bin/env bash

URL="http://127.0.0.1:8080/api/v1/send"
KEY="change-me-super-secret"
GROUP="monitoring"
COUNT="${1:-30}"

ok=0
fail=0

for i in $(seq 1 "$COUNT"); do
  resp="$(
    curl -s -w " HTTP_STATUS:%{http_code}" -X POST "$URL" \
      -H "X-Relay-Key: $KEY" \
      -H "Content-Type: application/json" \
      -d "{
        \"group\": \"$GROUP\",
        \"parse_mode\": \"HTML\",
        \"text\": \"<b>🔥 Queue test #$i</b>\n\n<code>$(date '+%Y-%m-%d %H:%M:%S')</code>\"
      }"
  )"

  code="${resp##*HTTP_STATUS:}"
  body="${resp% HTTP_STATUS:*}"

  if [ "$code" = "200" ]; then
    ok=$((ok + 1))
    echo "[$i] OK $body"
  else
    fail=$((fail + 1))
    echo "[$i] FAIL status=$code body=$body"
  fi
done

echo
echo "Result: ok=$ok fail=$fail total=$COUNT"