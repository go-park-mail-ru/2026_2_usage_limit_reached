#!/usr/bin/env bash
set -uo pipefail

echo "=== 1) гость: GET /feed/settings"
curl -s -D - -o /tmp/settings.json http://127.0.0.1:8082/feed/settings | grep -i 'cache-control'
cat /tmp/settings.json; echo

echo "=== 2) гость: GET /feed?sort=new&attachment_type=video (флаг выключен → 403)"
curl -s -w '\nHTTP %{http_code}\n' 'http://127.0.0.1:8082/feed?sort=new&attachment_type=video'

echo "=== 3) гость: GET /feed?sort=new (без фильтра → 200)"
curl -s -o /dev/null -w 'HTTP %{http_code}\n' 'http://127.0.0.1:8082/feed?sort=new'

echo "=== 4) логин админом и включение флага (force_on на себя)"
jar=$(mktemp)
csrf=$(curl -s -c "$jar" http://127.0.0.1:8081/csrf | sed -E 's/.*"csrf_token":"([^"]+)".*/\1/')
curl -s -b "$jar" -c "$jar" -X POST http://127.0.0.1:8081/login \
  -H "Content-Type: application/json" -H "X-CSRF-Token: $csrf" \
  -d '{"login":"admin","password":"admin12345"}' -o /dev/null
token=$(grep '__Host-token' "$jar" | awk '{print $7}')
me=$(curl -s -H "Cookie: __Host-token=$token" http://127.0.0.1:8081/profile/me)
uid=$(echo "$me" | sed -E 's/.*"id":"([^"]+)".*/\1/')
if [ -z "$uid" ] || [ "$uid" = "$me" ]; then
  uid=$(curl -s -H "Cookie: __Host-token=$token" http://127.0.0.1:8081/profile/me | head -c 200)
fi
echo "профиль: $me"
echo "user id: $uid"

csrf_admin=$(curl -s -c "$jar" -b "$jar" http://127.0.0.1:8084/csrf | sed -E 's/.*"csrf_token":"([^"]+)".*/\1/')
echo "--- force_on:"
curl -s -H "Cookie: __Host-token=$token" -H "X-CSRF-Token: $csrf_admin" \
  -H "Content-Type: application/json" -X POST \
  -d "{\"user_id\":\"$uid\",\"force\":\"on\"}" \
  -w 'HTTP %{http_code}\n' \
  http://127.0.0.1:8084/onlineconf/admin/flags/content/feed_attachment_filter/force

echo "=== 5) ждём поллинг контента (15с)"
sleep 18

echo "=== 6) авторизованный: GET /feed/settings"
curl -s -H "Cookie: __Host-token=$token" http://127.0.0.1:8082/feed/settings; echo

echo "=== 7) авторизованный: GET /feed?sort=new&attachment_type=video (флаг включён → 200)"
curl -s -o /dev/null -w 'HTTP %{http_code}\n' -H "Cookie: __Host-token=$token" \
  'http://127.0.0.1:8082/feed?sort=new&attachment_type=video'

echo "=== 8) снимаем force_on"
curl -s -H "Cookie: __Host-token=$token" -H "X-CSRF-Token: $csrf_admin" \
  -X DELETE -w 'HTTP %{http_code}\n' \
  "http://127.0.0.1:8084/onlineconf/admin/flags/content/feed_attachment_filter/force/$uid"

rm -f "$jar"
