-- One row per sellable unit, for the `pg_skip_locked` adapter.
--
-- This is the claim pattern Postgres-backed job queues use (river, good_job,
-- graphile-worker), pointed at inventory instead of jobs. Stock stops being a
-- number on a hot row and becomes a pile of rows, so buyers contend only when
-- they happen to reach for the same ticket, and SKIP LOCKED means they never
-- wait even then.
--
-- The cost is storage and seeding: a 50,000-unit sale is 50,000 rows.
CREATE TABLE IF NOT EXISTS flashsale.stock_tickets (
    id         BIGSERIAL PRIMARY KEY,
    product_id BIGINT  NOT NULL REFERENCES flashsale.products(id),
    claimed    BOOLEAN NOT NULL DEFAULT false
);

-- Partial index: it covers only unclaimed rows, so it shrinks as the sale drains
-- rather than growing. Without the WHERE clause the claim query would scan an
-- index that is mostly sold tickets by the end of the run.
CREATE INDEX IF NOT EXISTS stock_tickets_unclaimed_idx
    ON flashsale.stock_tickets (product_id) WHERE NOT claimed;
