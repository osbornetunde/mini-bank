-- Add indexes for transaction pagination and filtering

-- Composite index for account_id and created_at (main query)
-- This supports ORDER BY created_at DESC and filtering by account_id
CREATE INDEX IF NOT EXISTS idx_transactions_account_created
ON transactions(account_id, created_at DESC);

-- Index for type filtering
-- This is a partial index to save space (only where type is not null)
CREATE INDEX IF NOT EXISTS idx_transactions_type
ON transactions(type) WHERE type IS NOT NULL;

-- The unique index on reference already exists from migration 003
-- CREATE UNIQUE INDEX idx_transactions_reference ON transactions(reference) WHERE reference IS NOT NULL;
