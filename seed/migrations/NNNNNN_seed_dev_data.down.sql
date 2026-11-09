-- Финансовые записи (payment_order, payment_attempt, donation) защищены триггерами от удаления,
-- поэтому откат dev-seed выполняется пересозданием базы (make reset), а не DELETE.
SELECT 1;
