DROP INDEX IF EXISTS flashsale.orders_idempotency_key_uniq;

ALTER TABLE flashsale.orders
    DROP COLUMN IF EXISTS idempotency_key;
