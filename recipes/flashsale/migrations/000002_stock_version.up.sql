-- Version column for the `pg_optimistic` adapter.
--
-- Optimistic locking reads (stock, version), then writes conditional on the
-- version being unchanged. A losing writer updates zero rows and retries,
-- which is cheap when contention is low and a retry storm when it is not.
ALTER TABLE flashsale.products
    ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 0;
