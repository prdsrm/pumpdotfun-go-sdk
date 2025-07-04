package pumpdotfunsdk

import (
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
)

func TestCalculateSellQuote(t *testing.T) {
	t.Parallel()

	bondingCurve := &BondingCurveData{
		VirtualSolReserves:   big.NewInt(1_000_000_000),
		VirtualTokenReserves: big.NewInt(10_000_000_000),
	}

	// Selling 1 token.
	tokenAmount := uint64(1_000_000_000)
	// 2% slippage.
	percentage := 0.98

	got := calculateSellQuote(tokenAmount, bondingCurve, percentage)

	x := new(big.Int).Mul(bondingCurve.VirtualSolReserves, big.NewInt(int64(tokenAmount)))
	y := new(big.Int).Add(bondingCurve.VirtualTokenReserves, big.NewInt(int64(tokenAmount)))
	expectedA := new(big.Int).Div(x, y)
	expectedFloat := new(big.Float).Mul(
		new(big.Float).SetInt(expectedA),
		big.NewFloat(percentage),
	)

	expectedFinal, _ := expectedFloat.Int(nil)

	require.Equal(t, expectedFinal, got)
}
