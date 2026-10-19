-- Автообновление updated_at при изменении строки
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Отклоняет UPDATE защищённых значений
CREATE OR REPLACE FUNCTION reject_update()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'table "%": update of immutable data is not allowed',
        TG_TABLE_NAME
        USING ERRCODE = 'integrity_constraint_violation';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Запрещает физическое удаление финансовых записей
CREATE OR REPLACE FUNCTION reject_delete()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'table "%": physical delete of financial records is not allowed',
        TG_TABLE_NAME
        USING ERRCODE = 'restrict_violation';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Конец расчётного периода по зафиксированному тарифу
CREATE OR REPLACE FUNCTION compute_subscription_period_end(
    p_period_start   TIMESTAMPTZ,
    p_interval_unit  TEXT,
    p_interval_count SMALLINT
) RETURNS TIMESTAMPTZ AS $$
BEGIN
    RETURN (
        (p_period_start AT TIME ZONE 'UTC')
        + CASE p_interval_unit
            WHEN 'day'   THEN make_interval(days  => p_interval_count)
            WHEN 'month' THEN make_interval(months => p_interval_count)
            WHEN 'year'  THEN make_interval(years  => p_interval_count)
          END
    ) AT TIME ZONE 'UTC';
END;
$$ LANGUAGE plpgsql IMMUTABLE;