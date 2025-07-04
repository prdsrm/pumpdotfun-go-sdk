package pumpdotfunsdk

import (
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetBondingCurveAndAssociatedBondingCurve(t *testing.T) {
	t.Parallel()

	mint := solana.NewWallet().PublicKey()

	bondingCurveKeys, err := getBondingCurveAndAssociatedBondingCurve(mint)
	require.NoError(t, err)
	require.NotNil(t, bondingCurveKeys)
	require.False(t, bondingCurveKeys.BondingCurve.IsZero())
	require.False(t, bondingCurveKeys.AssociatedBondingCurve.IsZero())
}
