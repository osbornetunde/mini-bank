-- 1. CLEANUP (Fresh Start)
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS accounts CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- 2. CREATE USERS
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  first_name VARCHAR(255) NOT NULL,
  last_name VARCHAR(255) NOT NULL,
  email VARCHAR(255) NOT NULL UNIQUE,
  password VARCHAR(255) NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

-- 3. CREATE ACCOUNTS
CREATE TABLE accounts (
  id SERIAL PRIMARY KEY,
  user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  balance BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

-- 4. CREATE TRANSACTIONS
CREATE TABLE transactions (
  id BIGSERIAL PRIMARY KEY,
  account_id INT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  type VARCHAR(50) NOT NULL, -- deposit, withdraw, transfer
  amount BIGINT NOT NULL,
  reference VARCHAR(255),
  from_account_id INT NULL REFERENCES accounts(id),
  to_account_id INT NULL REFERENCES accounts(id),
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

-- 5. INDEXES
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_reference 
ON transactions(reference) WHERE reference IS NOT NULL;

CREATE INDEX idx_accounts_user_id ON accounts(user_id);
