#!/usr/bin/env bash
docker exec postgres psql -U patreon -d patreon -t \
  -c 'select count(*) as history_rows from oc_history;' \
  -c 'select version, dirty from schema_migrations;' \
  -c 'select scope, key, value from oc_config;' \
  -c 'select name from oc_scope order by name;'
