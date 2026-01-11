package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_CalculateTotalBalance(t *testing.T) {
	tests := []struct {
		name     string
		accounts []*Account
		expected int64
	}{
		{
			name:     "nil accounts returns zero",
			accounts: nil,
			expected: 0,
		},
		{
			name:     "empty accounts returns zero",
			accounts: []*Account{},
			expected: 0,
		},
		{
			name: "single positive balance",
			accounts: []*Account{
				{ID: 1, Balance: 1000},
			},
			expected: 1000,
		},
		{
			name: "multiple positive balances",
			accounts: []*Account{
				{ID: 1, Balance: 1000},
				{ID: 2, Balance: 2500},
				{ID: 3, Balance: 500},
			},
			expected: 4000,
		},
		{
			name: "mixed positive and negative balances (overdraft)",
			accounts: []*Account{
				{ID: 1, Balance: 1000},
				{ID: 2, Balance: -300}, // Overdraft
			},
			expected: 700,
		},
		{
			name: "all negative balances",
			accounts: []*Account{
				{ID: 1, Balance: -100},
				{ID: 2, Balance: -200},
			},
			expected: -300,
		},
		{
			name: "zero balance account",
			accounts: []*Account{
				{ID: 1, Balance: 0},
			},
			expected: 0,
		},
		{
			name: "large balances",
			accounts: []*Account{
				{ID: 1, Balance: 1000000000},  // 1 billion
				{ID: 2, Balance: 2000000000},  // 2 billion
			},
			expected: 3000000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{
				ID:        1,
				FirstName: "Test",
				LastName:  "User",
				Email:     "test@example.com",
				Accounts:  tt.accounts,
			}

			result := user.CalculateTotalBalance()

			assert.Equal(t, tt.expected, result)
		})
	}
}
