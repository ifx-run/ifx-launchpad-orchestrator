package pumpfun_test

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func TestBuildBuyInstructions_hasBuybackFeeRecipient(t *testing.T) {
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	mint := solana.MustPublicKeyFromBase58("CzLSujWBLFsSjncfkh59rUFqvafWcY5tzedWJSuypump")
	curve := pumpfun.BondingCurve{
		VirtualTokenReserves: 1_073_000_000_000_000,
		VirtualSolReserves:   30_000_000_000,
		RealTokenReserves:    793_100_000_000_000,
		TokenTotalSupply:     1_000_000_000_000_000,
		Creator:              solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
	}
	ixs, err := pumpfun.BuildBuyInstructions(pumpfun.BuildParams{
		Curve: curve, BaseMint: mint, User: user, BaseTokenProgram: solana.TokenProgramID,
		SpendableQuoteIn: 10_000_000, MinBaseOut: 1,
		PlatformFeePubkey: solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
		ComputeUnitLimit:  200_000, ComputeUnitPrice: 1_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	pumpIx := ixs[len(ixs)-1]
	accts := pumpIx.Accounts()
	if len(accts) != 18 {
		t.Fatalf("buy_exact_sol_in accounts=%d want 18", len(accts))
	}
	last := accts[len(accts)-1]
	if !last.IsWritable {
		t.Fatal("buyback fee recipient must be writable")
	}
}
