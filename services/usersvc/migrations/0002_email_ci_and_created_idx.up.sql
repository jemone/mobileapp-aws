-- 1) Делаем уникальность email регистронезависимой
ALTER TABLE app_user DROP CONSTRAINT IF EXISTS app_user_email_key;
CREATE UNIQUE INDEX IF NOT EXISTS app_user_email_lower_uq ON app_user (LOWER(email));

-- 2) Индекс для ускорения сортировки/выборок по времени создания
CREATE INDEX IF NOT EXISTS app_user_created_at_idx ON app_user (created_at DESC);
