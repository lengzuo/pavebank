--
-- Create bills table
--
CREATE TABLE IF NOT EXISTS bills (
    id SERIAL PRIMARY KEY,
    bill_id VARCHAR(64) NOT NULL,
    policy_type VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    UNIQUE(bill_id)
);
CREATE INDEX idx_bill_created_at_desc ON bills (created_at DESC);

--
-- Create line_items table
--
CREATE TABLE IF NOT EXISTS line_items (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(64) NOT NULL,
    bill_id VARCHAR(64) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    currency CHAR(3) NOT NULL,
    amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(uid)
);
CREATE INDEX idx_bill_id ON line_items (bill_id);
CREATE INDEX idx_line_item_created_at_desc ON line_items (created_at DESC);

