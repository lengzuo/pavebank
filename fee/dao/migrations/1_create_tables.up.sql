--
-- Create bills table
--
CREATE TABLE IF NOT EXISTS bills (
    id SERIAL PRIMARY KEY,
    bill_id VARCHAR(64) NOT NULL,
    policy_type VARCHAR(30) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    total_amount BIGINT NOT NULL DEFAULT 0,
    status CHAR(20) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    UNIQUE(bill_id)
);
CREATE INDEX idx_bills_status_created_at_desc ON bills (status, created_at DESC);

--
-- Create line_items table
--
CREATE TABLE IF NOT EXISTS line_items (
    id SERIAL PRIMARY KEY,
    line_item_id VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    bill_id VARCHAR(64) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(line_item_id)
);
CREATE INDEX idx_bill_id ON line_items (bill_id);
CREATE INDEX idx_line_item_created_at_desc ON line_items (created_at DESC);

