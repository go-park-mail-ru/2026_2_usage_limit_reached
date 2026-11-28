CREATE TABLE oc_scope (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_oc_scope_name UNIQUE (name),
    CONSTRAINT chk_oc_scope_name
        CHECK (length(trim(name)) > 0)
);

CREATE TRIGGER trg_oc_scope_set_updated_at
    BEFORE UPDATE ON oc_scope
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE oc_config (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope       TEXT NOT NULL,
    key         TEXT NOT NULL,
    value       JSONB NOT NULL,
    value_type  TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    updated_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_oc_config_scope_key UNIQUE (scope, key),
    CONSTRAINT fk_oc_config_scope
        FOREIGN KEY (scope)
        REFERENCES oc_scope (name)
        ON DELETE RESTRICT,
    CONSTRAINT fk_oc_config_updated_by
        FOREIGN KEY (updated_by)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_oc_config_key
        CHECK (length(trim(key)) > 0),
    CONSTRAINT chk_oc_config_value_type
        CHECK (value_type IN ('scalar', 'array', 'json'))
);

CREATE INDEX oc_config_scope_idx ON oc_config (scope);

CREATE TRIGGER trg_oc_config_set_updated_at
    BEFORE UPDATE ON oc_config
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE oc_flag (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope           TEXT NOT NULL,
    key             TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    rollout_percent SMALLINT NOT NULL DEFAULT 0,
    updated_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_oc_flag_scope_key UNIQUE (scope, key),
    CONSTRAINT fk_oc_flag_scope
        FOREIGN KEY (scope)
        REFERENCES oc_scope (name)
        ON DELETE RESTRICT,
    CONSTRAINT fk_oc_flag_updated_by
        FOREIGN KEY (updated_by)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_oc_flag_key
        CHECK (length(trim(key)) > 0),
    CONSTRAINT chk_oc_flag_rollout_percent
        CHECK (rollout_percent BETWEEN 0 AND 100)
);

CREATE INDEX oc_flag_scope_idx ON oc_flag (scope);

CREATE TRIGGER trg_oc_flag_set_updated_at
    BEFORE UPDATE ON oc_flag
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE oc_flag_user (
    flag_id     UUID NOT NULL,
    user_id     UUID NOT NULL,
    force       TEXT NOT NULL,
    created_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pk_oc_flag_user PRIMARY KEY (flag_id, user_id),
    CONSTRAINT fk_oc_flag_user_flag
        FOREIGN KEY (flag_id)
        REFERENCES oc_flag (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_oc_flag_user_user
        FOREIGN KEY (user_id)
        REFERENCES "user" (id)
        ON DELETE CASCADE,
    CONSTRAINT fk_oc_flag_user_created_by
        FOREIGN KEY (created_by)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_oc_flag_user_force
        CHECK (force IN ('on', 'off'))
);

CREATE INDEX oc_flag_user_user_id_idx ON oc_flag_user (user_id);

CREATE TRIGGER trg_oc_flag_user_set_updated_at
    BEFORE UPDATE ON oc_flag_user
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE oc_history (
    id          BIGSERIAL PRIMARY KEY,
    actor_id    UUID,
    entity_type TEXT NOT NULL,
    entity_id   UUID NOT NULL,
    action      TEXT NOT NULL,
    old_value   JSONB,
    new_value   JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_oc_history_actor
        FOREIGN KEY (actor_id)
        REFERENCES "user" (id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_oc_history_entity_type
        CHECK (entity_type IN ('config', 'flag', 'flag_user', 'scope')),
    CONSTRAINT chk_oc_history_action
        CHECK (action IN ('create', 'update', 'delete', 'force_on', 'force_off'))
);

CREATE INDEX oc_history_entity_type_entity_id_idx
    ON oc_history (entity_type, entity_id);

CREATE TRIGGER trg_oc_history_immutable
    BEFORE UPDATE ON oc_history
    FOR EACH ROW
    EXECUTE FUNCTION reject_update();

CREATE TRIGGER trg_oc_history_no_delete
    BEFORE DELETE ON oc_history
    FOR EACH ROW
    EXECUTE FUNCTION reject_delete();

INSERT INTO oc_scope (name, description) VALUES
    ('front',        'Настройки и флаги продуктового фронтенда'),
    ('monetization', 'Монолит донатов и платежей'),
    ('content',      'Контент-сервис: посты, ленты');

INSERT INTO oc_config (scope, key, value, value_type, description) VALUES
    ('monetization', 'payment.min_amount_minor', '100', 'scalar',
     'Минимальная сумма доната в копейках (100 = 1.00 RUB)');

INSERT INTO oc_flag (scope, key, description) VALUES
    ('content', 'feed_attachment_filter', 'Фильтр ленты по типу вложений');

-- Сиды обязаны попасть и в историю. Ревизия = max(oc_history.id), а клиент
-- при первом запросе шлёт known_revision = 0: если после чистого развертывания
-- история пуста, сервис ответит not_modified (0 == 0) и засеянные данные
-- не доедут до потребителей до первой ручной мутации.
INSERT INTO oc_history (entity_type, entity_id, action, new_value)
SELECT 'scope', id, 'create',
       jsonb_build_object('name', name, 'description', description)
  FROM oc_scope;

INSERT INTO oc_history (entity_type, entity_id, action, new_value)
SELECT 'config', id, 'create',
       jsonb_build_object('scope', scope, 'key', key, 'value', value,
                          'value_type', value_type, 'description', description)
  FROM oc_config;

INSERT INTO oc_history (entity_type, entity_id, action, new_value)
SELECT 'flag', id, 'create',
       jsonb_build_object('scope', scope, 'key', key,
                          'rollout_percent', rollout_percent,
                          'description', description)
  FROM oc_flag;
