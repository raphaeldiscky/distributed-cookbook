CREATE SCHEMA IF NOT EXISTS flashsale;

CREATE TABLE IF NOT EXISTS flashsale.products (
    id    BIGINT PRIMARY KEY,
    name  TEXT   NOT NULL,
    stock INT    NOT NULL
    -- NOTE: no CHECK (stock >= 0). The naive adapter MUST be able to
    -- write negative stock so the oversell demo is visible in Grafana.
);

CREATE TABLE IF NOT EXISTS flashsale.orders (
    id         BIGSERIAL PRIMARY KEY,
    product_id BIGINT      NOT NULL REFERENCES flashsale.products(id),
    qty        INT         NOT NULL CHECK (qty > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS orders_product_id_idx
    ON flashsale.orders (product_id);
