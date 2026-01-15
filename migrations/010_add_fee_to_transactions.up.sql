-- Add fee tracking columns to transactions table
ALTER TABLE transactions
ADD COLUMN fee_amount BIGINT DEFAULT 0 NOT NULL,
ADD COLUMN parent_transaction_id BIGINT,
ADD COLUMN is_fee_transaction BOOLEAN DEFAULT FALSE NOT NULL;

-- Foreign key to link fee transactions to their parent
ALTER TABLE transactions
ADD CONSTRAINT fk_parent_transaction
    FOREIGN KEY (parent_transaction_id)
    REFERENCES transactions(id)
    ON DELETE CASCADE;

-- Index for querying fee transactions
CREATE INDEX idx_transactions_fee_lookup ON transactions(parent_transaction_id)
    WHERE is_fee_transaction = TRUE;

COMMENT ON COLUMN transactions.fee_amount IS 'Fee charged for this transaction in cents (0 if no fee or if this is a fee transaction)';
COMMENT ON COLUMN transactions.parent_transaction_id IS 'ID of the parent transaction if this is a fee transaction';
COMMENT ON COLUMN transactions.is_fee_transaction IS 'TRUE if this row represents a fee charge, FALSE for regular transactions';
