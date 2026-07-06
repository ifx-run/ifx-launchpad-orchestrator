package pumpfun_test

import (
	"testing"

	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/gagliardetto/solana-go"
)

func TestBuildBuyInstructions_compileValid(t *testing.T) {
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	mint := solana.MustPublicKeyFromBase58("CzLSujWBLFsSjncfkh59rUFqvafWcY5tzedWJSuypump")

	curve := pumpfun.BondingCurve{
		VirtualTokenReserves: 1_073_000_000_000_000,
		VirtualSolReserves:   30_000_000_000,
		RealTokenReserves:    793_100_000_000_000,
		TokenTotalSupply:     1_000_000_000_000_000,
		Creator:              solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
	}
	feePub := solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM")

	ixs, err := pumpfun.BuildBuyInstructions(pumpfun.BuildParams{
		Curve:              curve,
		BaseMint:           mint,
		User:               user,
		BaseTokenProgram:   solana.TokenProgramID,
		SpendableQuoteIn:   10_000_000,
		MinBaseOut:         1,
		ServiceFeeLamports: 100_000,
		PlatformFeePubkey:  feePub,
		ComputeUnitLimit:   200_000,
		ComputeUnitPrice:   1_000,
	})
	if err != nil {
		t.Fatal(err)
	}

	bh := solana.MustHashFromBase58("EkSnNWid2cvwEVnVx9aBqawnmiCNiDgp3gUdkDPTKN1N")
	_, compiled, err := solpkg.CompileV0Tx(user, bh, ixs, nil)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := solana.TransactionFromBase64(compiled.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	msg := raw.Message
	for i, ix := range msg.Instructions {
		for j, acctIdx := range ix.Accounts {
			if int(acctIdx) >= len(msg.AccountKeys) {
				t.Fatalf("ix %d account ref %d: index %d >= %d keys", i, j, acctIdx, len(msg.AccountKeys))
			}
		}
		_, err := ix.ResolveInstructionAccounts(&msg)
		if err != nil {
			t.Fatalf("ix %d resolve: %v", i, err)
		}
	}

	ins, err := solpkg.InspectTransactionBase64(compiled.Transaction, compiled.TransactionSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(ins.Instructions) != ins.NumInstructions {
		t.Fatalf("instructions len=%d num=%d", len(ins.Instructions), ins.NumInstructions)
	}
	for _, ix := range ins.Instructions {
		if ix.Accounts == nil {
			t.Fatalf("ix %d accounts nil", ix.Index)
		}
	}
	raw2, err := raw.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw2) != compiled.TransactionSize {
		t.Fatalf("marshal size %d != %d", len(raw2), compiled.TransactionSize)
	}
}
