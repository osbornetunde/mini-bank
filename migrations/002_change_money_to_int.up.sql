-- Up Migration
ALTER TABLE accounts
    ALTER COLUMN balance TYPE BIGINT USING (balance * 100)::BIGINT;

ALTER TABLE transactions
    ALTER COLUMN amount TYPE BIGINT USING (amount * 100)::BIGINT;
