#!/usr/bin/env bash
set -uo pipefail

echo "1) текущее значение доната:"
curl -s http://127.0.0.1:8083/donations/settings; echo

echo "2) админ меняет конфиг на 5000 копеек (50 ₽) через API:"
jar=$(mktemp)
csrf=$(curl -s -c "$jar" http://127.0.0.1:8081/csrf | sed -E 's/.*"csrf_token":"([^"]+)".*/\1/')
curl -s -b "$jar" -c "$jar" -X POST http://127.0.0.1:8081/login \
  -H "Content-Type: application/json" -H "X-CSRF-Token: $csrf" \
  -d '{"login":"admin","password":"admin12345"}' -o /dev/null
token=$(grep '__Host-token' "$jar" | awk '{print $7}')

csrf_admin=$(curl -s -c "$jar" -b "$jar" http://127.0.0.1:8084/csrf | sed -E 's/.*"csrf_token":"([^"]+)".*/\1/')
curl -s -H "Cookie: __Host-token=$token" -H "X-CSRF-Token: $csrf_admin" \
  -H "Content-Type: application/json" -X PUT \
  -d '{"value":"5000","value_type":"scalar","description":"Минимальная сумма доната в копейках (50.00 RUB)"}' \
  -w '\nHTTP %{http_code}\n' \
  http://127.0.0.1:8084/onlineconf/admin/configs/monetization/payment.min_amount_minor

echo "3) ждём поллинг потребителя (15с) и смотрим снова:"
sleep 18
curl -s http://127.0.0.1:8083/donations/settings; echo

echo "4) возвращаем 100:"
curl -s -H "Cookie: __Host-token=$token" -H "X-CSRF-Token: $csrf_admin" \
  -H "Content-Type: application/json" -X PUT \
  -d '{"value":"100","value_type":"scalar","description":"Минимальная сумма доната в копейках (100 = 1.00 RUB)"}' \
  -o /dev/null -w 'HTTP %{http_code}\n' \
  http://127.0.0.1:8084/onlineconf/admin/configs/monetization/payment.min_amount_minor

sleep 18
echo "5) значение после отката:"
curl -s http://127.0.0.1:8083/donations/settings; echo

echo "6) история изменений (первые 3):"
curl -s -H "Cookie: __Host-token=$token" \
  'http://127.0.0.1:8084/onlineconf/admin/history?limit=3&entity_type=config&scope=monetization&key=payment.min_amount_minor' | head -c 900; echo

rm -f "$jar"
