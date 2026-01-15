CREATE TABLE fee_tiers (
    id SERIAL PRIMARY KEY,
    transaction_type VARCHAR(50) NOT NULL,  -- 'transfer', 'withdraw'
    min_amount BIGINT NOT NULL,             -- minimum amount in cents (inclusive)
    max_amount BIGINT,                      -- maximum amount in cents (exclusive), NULL = unlimited
    fee_type VARCHAR(20) NOT NULL,          -- 'flat', 'percentage', 'combined'
    flat_fee BIGINT,                        -- flat fee in cents (NULL if not applicable)
    percentage_fee DECIMAL(5,4),            -- percentage as decimal (e.g., 0.0250 = 2.5%), NULL if not applicable
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT valid_fee_type CHECK (fee_type IN ('flat', 'percentage', 'combined')),
    CONSTRAINT valid_amounts CHECK (min_amount >= 0 AND (max_amount IS NULL OR max_amount > min_amount)),
    CONSTRAINT valid_flat_fee CHECK ((fee_type != 'flat' AND fee_type != 'combined') OR flat_fee IS NOT NULL),
    CONSTRAINT valid_percentage CHECK ((fee_type != 'percentage' AND fee_type != 'combined') OR percentage_fee IS NOT NULL)
);

-- Index for fast fee lookups
CREATE INDEX idx_fee_tiers_lookup ON fee_tiers(transaction_type, is_active, min_amount, max_amount);

-- Seed with default tier structure
INSERT INTO fee_tiers (transaction_type, min_amount, max_amount, fee_type, flat_fee, percentage_fee) VALUES
    -- Transfer fees (P2P)
    ('transfer', 0, 10000, 'flat', 25, NULL),                    -- < $100: $0.25
    ('transfer', 10000, 100000, 'flat', 50, NULL),               -- $100-$1000: $0.50
    ('transfer', 100000, 1000000, 'combined', 100, 0.0050),      -- $1,000-$10,000: $1.00 + 0.5%
    ('transfer', 1000000, NULL, 'percentage', NULL, 0.0100),     -- > $10,000: 1%

    -- Withdrawal fees (ATM-style)
    ('withdraw', 0, 10000, 'flat', 50, NULL),                    -- < $100: $0.50
    ('withdraw', 10000, 50000, 'flat', 100, NULL),               -- $100-$500: $1.00
    ('withdraw', 50000, NULL, 'combined', 200, 0.0025);          -- > $500: $2.00 + 0.25%

COMMENT ON TABLE fee_tiers IS 'Defines fee structures for different transaction types and amount ranges';
