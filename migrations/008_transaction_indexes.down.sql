-- Remove transaction indexes

DROP INDEX IF EXISTS idx_transactions_account_created;
DROP INDEX IF EXISTS idx_transactions_type;
