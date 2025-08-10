-- Откат: убираем функциональный уникальный индекс и индекс по времени
DROP INDEX IF EXISTS app_user_email_lower_uq;
DROP INDEX IF EXISTS app_user_created_at_idx;

-- Возвращаем прежнее ограничение уникальности (чувствительное к регистру)
ALTER TABLE app_user ADD CONSTRAINT app_user_email_key UNIQUE (email);
