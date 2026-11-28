#!/usr/bin/env bash
set -uo pipefail

jar=$(mktemp)
csrf=$(curl -s -c "$jar" http://127.0.0.1:8081/csrf | sed -E 's/.*"csrf_token":"([^"]+)".*/\1/')
echo "csrf получен: ${csrf:0:12}..."

echo "--- login admin:"
curl -s -b "$jar" -c "$jar" -X POST http://127.0.0.1:8081/login \
  -H "Content-Type: application/json" -H "X-CSRF-Token: $csrf" \
  -d '{"login":"admin","password":"admin12345"}' -w '\nHTTP %{http_code}\n'

token=$(grep '__Host-token' "$jar" | awk '{print $7}')
echo "token: ${token:0:16}..."

echo "--- /onlineconf/admin/access:"
curl -s -H "Cookie: __Host-token=$token" -w '\nHTTP %{http_code}\n' \
  http://127.0.0.1:8084/onlineconf/admin/access

echo "--- /onlineconf/admin/scopes:"
curl -s -H "Cookie: __Host-token=$token" -w '\nHTTP %{http_code}\n' \
  http://127.0.0.1:8084/onlineconf/admin/scopes

echo "--- /onlineconf/admin/configs?scope=monetization&q=min:"
curl -s -H "Cookie: __Host-token=$token" -w '\nHTTP %{http_code}\n' \
  'http://127.0.0.1:8084/onlineconf/admin/configs?scope=monetization&q=min'

echo "--- /onlineconf/admin/flags?scope=content:"
curl -s -H "Cookie: __Host-token=$token" -w '\nHTTP %{http_code}\n' \
  'http://127.0.0.1:8084/onlineconf/admin/flags?scope=content'

echo "--- /onlineconf/admin/history?limit=3:"
curl -s -H "Cookie: __Host-token=$token" -w '\nHTTP %{http_code}\n' \
  'http://127.0.0.1:8084/onlineconf/admin/history?limit=3'

echo "--- /onlineconf/admin/flags/check (флаг выключен, 0%):"
curl -s -H "Cookie: __Host-token=$token" -H "Content-Type: application/json" \
  -X POST -d '{"scope":"content","key":"feed_attachment_filter","user_id":"11111111-1111-1111-1111-111111111111"}' \
  -w '\nHTTP %{http_code}\n' http://127.0.0.1:8084/onlineconf/admin/flags/check

rm -f "$jar"
