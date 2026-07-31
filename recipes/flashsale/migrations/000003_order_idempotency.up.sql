-- Idempotency key for the `token_queue` adapter's async order writer.
--
-- Kafka delivers at least once, so a redelivered message would insert the same
-- order twice. The consumer inserts with ON CONFLICT DO NOTHING against this
-- key, which makes replay safe. Deduplication windows and the general shape of
-- this problem are the `idempotency` recipe's subject; this column is the
-- minimum needed to keep the flashsale numbers honest.
--
-- Nullable on purpose: Postgres allows many NULLs in a UNIQUE index, so the
-- eight synchronous adapters keep inserting without a key and stay unaffected.
ALTER TABLE flashsale.orders
    ADD COLUMN IF NOT EXISTS idempotency_key UUID;

CREATE UNIQUE INDEX IF NOT EXISTS orders_idempotency_key_uniq
    ON flashsale.orders (idempotency_key);
