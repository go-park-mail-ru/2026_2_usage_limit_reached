-- Тестовые учётные записи.
INSERT INTO "user" (id, username, nickname, email, password_hash, status)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'alice', 'Alice',
     'alice@example.com',
     '$2b$12$KIXQ7v1eYQwR9m0y0m8g2.z3xQdG5wS4b8fQ0y8mVYQwR9m0y0m8g',
     'active'),
    ('00000000-0000-0000-0000-000000000002', 'bob', 'Bob',
     'bob@example.com',
     '$2b$12$KIXQ7v1eYQwR9m0y0m8g2.z3xQdG5wS4b8fQ0y8mVYQwR9m0y0m8g',
     'active'),
    ('00000000-0000-0000-0000-000000000003', 'carol', 'Carol',
     'carol@example.com',
     '$2b$12$KIXQ7v1eYQwR9m0y0m8g2.z3xQdG5wS4b8fQ0y8mVYQwR9m0y0m8g',
     'active');

-- Carol -- автор.
INSERT INTO author (id, bio, category)
VALUES
    ('00000000-0000-0000-0000-000000000003',
     'Пишу про иллюстрацию и цифровой арт.',
     'art');

-- Один уровень подписки и один тарифный план к нему.
INSERT INTO subscription_type (id, author_id, name, description, level)
VALUES
    ('00000000-0000-0000-0000-000000000101',
     '00000000-0000-0000-0000-000000000003',
     'Basic',
     'Доступ к закрытым постам и раннему контенту.',
     1);

INSERT INTO billing_plan (id, subscription_type_id, interval_unit, interval_count,
                           amount_minor, currency)
VALUES
    ('00000000-0000-0000-0000-000000000201',
     '00000000-0000-0000-0000-000000000101',
     'month', 1, 49900, 'RUB');

-- Общедоступный пост.
INSERT INTO post (id, author_id, title, body, status, published_at)
VALUES
    ('00000000-0000-0000-0000-000000000301',
     '00000000-0000-0000-0000-000000000003',
     'Добро пожаловать на мою страницу',
     'Здесь буду выкладывать иллюстрации и разборы процесса.',
     'published', now());

-- Пост, закрытый уровнем подписки Basic.
INSERT INTO post (id, author_id, title, body, status, published_at)
VALUES
    ('00000000-0000-0000-0000-000000000302',
     '00000000-0000-0000-0000-000000000003',
     'Эксклюзив для подписчиков',
     'Пошаговый разбор последней работы -- доступно по подписке Basic.',
     'published', now());

INSERT INTO post_access (post_id, subscription_type_id)
VALUES
    ('00000000-0000-0000-0000-000000000302',
     '00000000-0000-0000-0000-000000000101');