package core

import "time"

type FeeType string

const (
	FeeTypeFlat       FeeType = "flat"
	FeeTypePercentage FeeType = "percentage"
	FeeTypeCombined   FeeType = "combined"
)

// FeeTier represents a fee rule for a specific transaction type and amount range
type FeeTier struct {
	ID              int
	TransactionType string   // 'transfer', 'withdraw'
	MinAmount       int64    // in cents
	MaxAmount       *int64   // in cents, nil = unlimited
	FeeType         FeeType
	FlatFee         *int64   // in cents
	PercentageFee   *float64 // as decimal (e.g., 0.025 = 2.5%)
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// FeeCalculation represents the result of calculating a fee
type FeeCalculation struct {
	TransactionAmount int64
	FeeAmount         int64
	FeeTier           *FeeTier
	Breakdown         string // e.g., "$1.00 flat + 0.5% of $5,000 = $1.00 + $25.00"
}

// CalculateFee computes the fee based on the tier's rules
func (ft *FeeTier) CalculateFee(amount int64) int64 {
	var fee int64

	switch ft.FeeType {
	case FeeTypeFlat:
		fee = *ft.FlatFee
	case FeeTypePercentage:
		fee = int64(float64(amount) * (*ft.PercentageFee))
	case FeeTypeCombined:
		percentagePart := int64(float64(amount) * (*ft.PercentageFee))
		fee = *ft.FlatFee + percentagePart
	}

	return fee
}

// Matches checks if this tier applies to the given amount
func (ft *FeeTier) Matches(amount int64) bool {
	if amount < ft.MinAmount {
		return false
	}
	if ft.MaxAmount != nil && amount >= *ft.MaxAmount {
		return false
	}
	return true
}

// TransferResult represents the result of a transfer operation including fee info
type TransferResult struct {
	FromAccount *Account
	ToAccount   *Account
	FeeAmount   int64
	Reference   string
}

// WithdrawResult represents the result of a withdrawal operation including fee info
type WithdrawResult struct {
	Account   *Account
	FeeAmount int64
	Reference string
}
