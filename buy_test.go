package pumpdotfunsdk

import (
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
)

func TestCalculateBuyQuote(t *testing.T) {
	t.Parallel()

	bondingCurve := &BondingCurveData{
		VirtualSolReserves:   big.NewInt(1_000_000_000),
		VirtualTokenReserves: big.NewInt(10_000_000_000),
	}

	solAmount := uint64(1_000_000_000)
	// 2% slippage.
	percentage := 0.98

	tokens := calculateBuyQuote(solAmount, bondingCurve, percentage)

	invariant := new(big.Int).Mul(bondingCurve.VirtualSolReserves, bondingCurve.VirtualTokenReserves)
	newSolReserves := new(big.Int).Add(bondingCurve.VirtualSolReserves, big.NewInt(int64(solAmount)))
	newTokenReserves := new(big.Int).Div(invariant, newSolReserves)
	expectedTokens := new(big.Int).Sub(bondingCurve.VirtualTokenReserves, newTokenReserves)
	expectedTokensFloat := new(big.Float).Mul(
		new(big.Float).SetInt(expectedTokens),
		big.NewFloat(percentage),
	)

	expectedTokensFinal, _ := expectedTokensFloat.Int(nil)

	require.Equal(t, expectedTokensFinal, tokens)
}

func TestConvertSlippageBasisPointsToPercentage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		slippageBasisPoints uint
		expected            float64
	}{
		{"Zero slippage", 0, 1.0},
		{"Two percent slippage", 200, 0.98},
		{"Full slippage", 10000, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := convertSlippageBasisPointsToPercentage(tt.slippageBasisPoints)
			require.InDelta(t, tt.expected, got, 0.0001)
		})
	}
}
