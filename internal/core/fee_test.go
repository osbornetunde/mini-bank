package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T {
	return &v
}

func TestFeeTier_CalculateFee(t *testing.T) {
	tests := []struct {
		name     string
		tier     FeeTier
		amount   int64
		expected int64
	}{
		{
			name: "flat fee",
			tier: FeeTier{
				FeeType: FeeTypeFlat,
				FlatFee: ptr(int64(50)), // $0.50
			},
			amount:   10000, // $100
			expected: 50,    // $0.50
		},
		{
			name: "percentage fee - 1%",
			tier: FeeTier{
				FeeType:       FeeTypePercentage,
				PercentageFee: ptr(0.01), // 1%
			},
			amount:   10000, // $100
			expected: 100,   // $1.00
		},
		{
			name: "percentage fee - 2.5%",
			tier: FeeTier{
				FeeType:       FeeTypePercentage,
				PercentageFee: ptr(0.025), // 2.5%
			},
			amount:   20000, // $200
			expected: 500,   // $5.00
		},
		{
			name: "combined fee - flat + percentage",
			tier: FeeTier{
				FeeType:       FeeTypeCombined,
				FlatFee:       ptr(int64(100)), // $1.00
				PercentageFee: ptr(0.005),      // 0.5%
			},
			amount:   500000, // $5,000
			expected: 2600,   // $1.00 + ($5,000 * 0.5%) = $1.00 + $25.00 = $26.00
		},
		{
			name: "flat fee with zero amount",
			tier: FeeTier{
				FeeType: FeeTypeFlat,
				FlatFee: ptr(int64(25)),
			},
			amount:   0,
			expected: 25, // Still charges flat fee
		},
		{
			name: "percentage fee with small amount",
			tier: FeeTier{
				FeeType:       FeeTypePercentage,
				PercentageFee: ptr(0.01),
			},
			amount:   50, // $0.50
			expected: 0,  // Rounds down to 0
		},
		{
			name: "large amount percentage fee",
			tier: FeeTier{
				FeeType:       FeeTypePercentage,
				PercentageFee: ptr(0.01), // 1%
			},
			amount:   10000000, // $100,000
			expected: 100000,   // $1,000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tier.CalculateFee(tt.amount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFeeTier_Matches(t *testing.T) {
	tests := []struct {
		name     string
		tier     FeeTier
		amount   int64
		expected bool
	}{
		{
			name: "amount within range",
			tier: FeeTier{
				MinAmount: 10000,           // $100
				MaxAmount: ptr(int64(100000)), // $1,000
			},
			amount:   50000, // $500
			expected: true,
		},
		{
			name: "amount at minimum (inclusive)",
			tier: FeeTier{
				MinAmount: 10000,
				MaxAmount: ptr(int64(100000)),
			},
			amount:   10000,
			expected: true,
		},
		{
			name: "amount at maximum (exclusive)",
			tier: FeeTier{
				MinAmount: 10000,
				MaxAmount: ptr(int64(100000)),
			},
			amount:   100000,
			expected: false, // Max is exclusive
		},
		{
			name: "amount below minimum",
			tier: FeeTier{
				MinAmount: 10000,
				MaxAmount: ptr(int64(100000)),
			},
			amount:   5000,
			expected: false,
		},
		{
			name: "amount above maximum",
			tier: FeeTier{
				MinAmount: 10000,
				MaxAmount: ptr(int64(100000)),
			},
			amount:   150000,
			expected: false,
		},
		{
			name: "nil max amount (unlimited)",
			tier: FeeTier{
				MinAmount: 1000000, // $10,000
				MaxAmount: nil,
			},
			amount:   50000000, // $500,000
			expected: true,
		},
		{
			name: "zero minimum with nil max",
			tier: FeeTier{
				MinAmount: 0,
				MaxAmount: nil,
			},
			amount:   1,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tier.Matches(tt.amount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransferResult(t *testing.T) {
	result := TransferResult{
		FromAccount: &Account{ID: 1, Balance: 900},
		ToAccount:   &Account{ID: 2, Balance: 1100},
		FeeAmount:   50,
		Reference:   "test-ref-123",
	}

	assert.Equal(t, int64(900), result.FromAccount.Balance)
	assert.Equal(t, int64(1100), result.ToAccount.Balance)
	assert.Equal(t, int64(50), result.FeeAmount)
	assert.Equal(t, "test-ref-123", result.Reference)
}

func TestWithdrawResult(t *testing.T) {
	result := WithdrawResult{
		Account:   &Account{ID: 1, Balance: 900},
		FeeAmount: 50,
		Reference: "wd-ref-123",
	}

	assert.Equal(t, int64(900), result.Account.Balance)
	assert.Equal(t, int64(50), result.FeeAmount)
	assert.Equal(t, "wd-ref-123", result.Reference)
}
