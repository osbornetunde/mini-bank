-- Down Migration
ALTER TABLE accounts
    ALTER COLUMN balance TYPE NUMERIC(20,2) USING (balance / 100.0);
ALTER TABLE transactions
    ALTER COLUMN amount TYPE NUMERIC(20,2) USING (amount / 100.0);