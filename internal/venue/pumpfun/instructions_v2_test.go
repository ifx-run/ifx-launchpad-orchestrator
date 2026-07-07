package pumpfun_test

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func TestBuildBuyV2Instructions_compileValid(t *testing.T) {
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	mint := solana.MustPublicKeyFromBase58("CzLSujWBLFsSjncfkh59rUFqvafWcY5tzedWJSuypump")
	usdc := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	feePub := solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM")
	feeATA, _, _ := solana.FindAssociatedTokenAddress(feePub, usdc)

	curve := pumpfun.BondingCurve{
		VirtualTokenReserves: 1_073_000_000_000_000,
		VirtualSolReserves:   30_000_000_000,
		RealTokenReserves:    793_100_000_000_000,
		TokenTotalSupply:     1_000_000_000_000_000,
		Creator:              solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
		QuoteMint:            usdc,
	}

	ixs, err := pumpfun.BuildBuyV2Instructions(pumpfun.BuildParams{
		Curve:               curve,
		BaseMint:            mint,
		User:                user,
		BaseTokenProgram:    solana.TokenProgramID,
		QuoteMint:           usdc,
		QuoteTokenProgram:   solana.TokenProgramID,
		SpendableQuoteIn:    1_000_000,
		MinBaseOut:          1,
		ServiceFeeQuote:     5_000,
		PlatformFeePubkey:   feePub,
		PlatformFeeQuoteATA: feeATA,
		ComputeUnitLimit:    200_000,
		ComputeUnitPrice:    1_000,
	})
	if err != nil {
		t.Fatal(err)
	}

	bh := solana.MustHashFromBase58("EkSnNWid2cvwEVnVx9aBqawnmiCNiDgp3gUdkDPTKN1N")
	_, compiled, err := solpkg.CompileV0Tx(user, bh, ixs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.TransactionSize == 0 {
		t.Fatal("expected non-zero tx size")
	}
}

func TestBuildSellV2Instructions_compileValid(t *testing.T) {
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	mint := solana.MustPublicKeyFromBase58("CzLSujWBLFsSjncfkh59rUFqvafWcY5tzedWJSuypump")
	usdc := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	feePub := solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM")
	feeATA, _, _ := solana.FindAssociatedTokenAddress(feePub, usdc)

	curve := pumpfun.BondingCurve{
		VirtualTokenReserves: 1_073_000_000_000_000,
		VirtualSolReserves:   30_000_000_000,
		RealTokenReserves:    793_100_000_000_000,
		TokenTotalSupply:     1_000_000_000_000_000,
		Creator:              solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
		QuoteMint:            usdc,
	}

	ixs, err := pumpfun.BuildSellV2Instructions(pumpfun.BuildParams{
		Curve:               curve,
		BaseMint:            mint,
		User:                user,
		BaseTokenProgram:    solana.TokenProgramID,
		QuoteMint:           usdc,
		QuoteTokenProgram:   solana.TokenProgramID,
		BaseAmountIn:        1_000_000,
		MinQuoteOut:         1,
		ServiceFeeQuote:     100,
		PlatformFeePubkey:   feePub,
		PlatformFeeQuoteATA: feeATA,
		ComputeUnitLimit:    200_000,
		ComputeUnitPrice:    1_000,
	})
	if err != nil {
		t.Fatal(err)
	}

	bh := solana.MustHashFromBase58("EkSnNWid2cvwEVnVx9aBqawnmiCNiDgp3gUdkDPTKN1N")
	_, compiled, err := solpkg.CompileV0Tx(user, bh, ixs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.TransactionSize == 0 {
		t.Fatal("expected non-zero tx size")
	}
}
