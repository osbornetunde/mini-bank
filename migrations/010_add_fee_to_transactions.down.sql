ALTER TABLE transactions DROP CONSTRAINT IF EXISTS fk_parent_transaction;
DROP INDEX IF EXISTS idx_transactions_fee_lookup;
ALTER TABLE transactions
    DROP COLUMN IF EXISTS is_fee_transaction,
    DROP COLUMN IF EXISTS parent_transaction_id,
    DROP COLUMN IF EXISTS fee_amount;
