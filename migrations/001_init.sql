-- accounts table
CREATE TABLE IF NOT EXISTS accounts (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  balance NUMERIC(20,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

-- transactions table
CREATE TABLE IF NOT EXISTS transactions (
  id BIGSERIAL PRIMARY KEY,
  account_id INT NOT NULL REFERENCES accounts(id),
  type VARCHAR(50) NOT NULL, -- deposit, withdraw, transfer_debit, transfer_credit
  amount NUMERIC(20,2) NOT NULL,
  reference VARCHAR(255),
  from_account_id INT NULL REFERENCES accounts(id),
  to_account_id INT NULL REFERENCES accounts(id),
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

-- unique index on reference to support idempotency if desired (nullable)
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_reference ON transactions(reference) WHERE reference IS NOT NULL;
