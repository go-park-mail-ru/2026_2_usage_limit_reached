-- Идемпотентно досеивает историю для уже развёрнутой БД
-- (эквивалент блока сидов из 000016_create_onlineconf.up.sql).
INSERT INTO oc_history (entity_type, entity_id, action, new_value)
SELECT 'scope', id, 'create',
       jsonb_build_object('name', name, 'description', description)
  FROM oc_scope
 WHERE NOT EXISTS (SELECT 1 FROM oc_history WHERE entity_type = 'scope');

INSERT INTO oc_history (entity_type, entity_id, action, new_value)
SELECT 'config', id, 'create',
       jsonb_build_object('scope', scope, 'key', key, 'value', value,
                          'value_type', value_type, 'description', description)
  FROM oc_config
 WHERE NOT EXISTS (SELECT 1 FROM oc_history WHERE entity_type = 'config');

INSERT INTO oc_history (entity_type, entity_id, action, new_value)
SELECT 'flag', id, 'create',
       jsonb_build_object('scope', scope, 'key', key,
                          'rollout_percent', rollout_percent,
                          'description', description)
  FROM oc_flag
 WHERE NOT EXISTS (SELECT 1 FROM oc_history WHERE entity_type = 'flag');
