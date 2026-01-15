package service

import (
	"context"
	"fmt"

	"mini-bank/internal/core"
	"mini-bank/internal/storage"
)

// CalculateFee determines the applicable fee for a transaction
func (s *service) CalculateFee(ctx context.Context, transactionType string, amount int64) (*core.FeeCalculation, error) {
	tier, err := s.store.GetApplicableFeeTier(ctx, transactionType, amount)
	if err != nil {
		// If no fee rule found, return zero fee (backwards compatible)
		if err == storage.ErrFeeRuleNotFound {
			return &core.FeeCalculation{
				TransactionAmount: amount,
				FeeAmount:         0,
				FeeTier:           nil,
				Breakdown:         "No fee applied",
			}, nil
		}
		return nil, err
	}

	feeAmount := tier.CalculateFee(amount)

	breakdown := s.buildFeeBreakdown(tier, amount, feeAmount)

	return &core.FeeCalculation{
		TransactionAmount: amount,
		FeeAmount:         feeAmount,
		FeeTier:           tier,
		Breakdown:         breakdown,
	}, nil
}

func (s *service) buildFeeBreakdown(tier *core.FeeTier, amount, feeAmount int64) string {
	switch tier.FeeType {
	case core.FeeTypeFlat:
		return fmt.Sprintf("Flat fee: $%.2f", float64(feeAmount)/100)
	case core.FeeTypePercentage:
		return fmt.Sprintf("%.2f%% of $%.2f = $%.2f",
			*tier.PercentageFee*100,
			float64(amount)/100,
			float64(feeAmount)/100)
	case core.FeeTypeCombined:
		flatPart := *tier.FlatFee
		percentPart := feeAmount - flatPart
		return fmt.Sprintf("$%.2f flat + %.2f%% of $%.2f = $%.2f + $%.2f = $%.2f",
			float64(flatPart)/100,
			*tier.PercentageFee*100,
			float64(amount)/100,
			float64(flatPart)/100,
			float64(percentPart)/100,
			float64(feeAmount)/100)
	}
	return ""
}

// CreateFeeTier creates a new fee tier rule
func (s *service) CreateFeeTier(ctx context.Context, tier *core.FeeTier) (*core.FeeTier, error) {
	return s.store.CreateFeeTier(ctx, tier)
}

// ListFeeTiers returns all fee tiers, optionally filtered by transaction type
func (s *service) ListFeeTiers(ctx context.Context, transactionType *string, activeOnly bool) ([]*core.FeeTier, error) {
	return s.store.ListFeeTiers(ctx, transactionType, activeOnly)
}

// UpdateFeeTier updates an existing fee tier
func (s *service) UpdateFeeTier(ctx context.Context, tier *core.FeeTier) error {
	return s.store.UpdateFeeTier(ctx, tier)
}

// DeleteFeeTier marks a fee tier as inactive
func (s *service) DeleteFeeTier(ctx context.Context, id int) error {
	return s.store.DeleteFeeTier(ctx, id)
}
