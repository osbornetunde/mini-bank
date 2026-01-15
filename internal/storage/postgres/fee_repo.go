package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"mini-bank/internal/core"
	"mini-bank/internal/storage"
)

// CreateFeeTier creates a new fee tier rule
func (r *Repo) CreateFeeTier(ctx context.Context, tier *core.FeeTier) (*core.FeeTier, error) {
	query := `
		INSERT INTO fee_tiers (transaction_type, min_amount, max_amount, fee_type, flat_fee, percentage_fee, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(
		ctx, query,
		tier.TransactionType,
		tier.MinAmount,
		tier.MaxAmount,
		tier.FeeType,
		tier.FlatFee,
		tier.PercentageFee,
		tier.IsActive,
	).Scan(&tier.ID, &tier.CreatedAt, &tier.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create fee tier: %w", err)
	}

	return tier, nil
}

// GetApplicableFeeTier finds the fee tier that applies to a transaction
func (r *Repo) GetApplicableFeeTier(ctx context.Context, transactionType string, amount int64) (*core.FeeTier, error) {
	query := `
		SELECT id, transaction_type, min_amount, max_amount, fee_type, flat_fee, percentage_fee, is_active, created_at, updated_at
		FROM fee_tiers
		WHERE transaction_type = $1
		  AND is_active = TRUE
		  AND min_amount <= $2
		  AND (max_amount IS NULL OR max_amount > $2)
		ORDER BY min_amount DESC
		LIMIT 1`

	tier := &core.FeeTier{}
	err := r.db.QueryRowContext(ctx, query, transactionType, amount).Scan(
		&tier.ID,
		&tier.TransactionType,
		&tier.MinAmount,
		&tier.MaxAmount,
		&tier.FeeType,
		&tier.FlatFee,
		&tier.PercentageFee,
		&tier.IsActive,
		&tier.CreatedAt,
		&tier.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, storage.ErrFeeRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get fee tier: %w", err)
	}

	return tier, nil
}

// ListFeeTiers returns all fee tiers, optionally filtered by transaction type
func (r *Repo) ListFeeTiers(ctx context.Context, transactionType *string, activeOnly bool) ([]*core.FeeTier, error) {
	query := `
		SELECT id, transaction_type, min_amount, max_amount, fee_type, flat_fee, percentage_fee, is_active, created_at, updated_at
		FROM fee_tiers
		WHERE ($1::VARCHAR IS NULL OR transaction_type = $1)
		  AND ($2 = FALSE OR is_active = TRUE)
		ORDER BY transaction_type, min_amount`

	rows, err := r.db.QueryContext(ctx, query, transactionType, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to list fee tiers: %w", err)
	}
	defer rows.Close()

	var tiers []*core.FeeTier
	for rows.Next() {
		tier := &core.FeeTier{}
		err := rows.Scan(
			&tier.ID,
			&tier.TransactionType,
			&tier.MinAmount,
			&tier.MaxAmount,
			&tier.FeeType,
			&tier.FlatFee,
			&tier.PercentageFee,
			&tier.IsActive,
			&tier.CreatedAt,
			&tier.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan fee tier: %w", err)
		}
		tiers = append(tiers, tier)
	}

	return tiers, nil
}

// UpdateFeeTier updates an existing fee tier
func (r *Repo) UpdateFeeTier(ctx context.Context, tier *core.FeeTier) error {
	query := `
		UPDATE fee_tiers
		SET transaction_type = $2,
			min_amount = $3,
			max_amount = $4,
			fee_type = $5,
			flat_fee = $6,
			percentage_fee = $7,
			is_active = $8,
			updated_at = NOW()
		WHERE id = $1`

	result, err := r.db.ExecContext(
		ctx, query,
		tier.ID,
		tier.TransactionType,
		tier.MinAmount,
		tier.MaxAmount,
		tier.FeeType,
		tier.FlatFee,
		tier.PercentageFee,
		tier.IsActive,
	)

	if err != nil {
		return fmt.Errorf("failed to update fee tier: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return storage.ErrFeeRuleNotFound
	}

	return nil
}

// DeleteFeeTier marks a fee tier as inactive
func (r *Repo) DeleteFeeTier(ctx context.Context, id int) error {
	query := `UPDATE fee_tiers SET is_active = FALSE, updated_at = NOW() WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete fee tier: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return storage.ErrFeeRuleNotFound
	}

	return nil
}

// ListUserAccounts returns all accounts for a given user
func (r *Repo) ListUserAccounts(ctx context.Context, userID int) ([]*core.Account, error) {
	query := `
		SELECT id, user_id, balance, overdraft_limit, created_at
		FROM accounts
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*core.Account
	for rows.Next() {
		acc := &core.Account{}
		err := rows.Scan(
			&acc.ID,
			&acc.UserID,
			&acc.Balance,
			&acc.OverdraftLimit,
			&acc.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, acc)
	}

	return accounts, nil
}
