package pumpfun_test

import (
	"encoding/base64"
	"testing"

	"github.com/gagliardetto/solana-go"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func TestBuildBuyInstructions_dumpLayout(t *testing.T) {
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
		SpendableQuoteIn: 10_000_000, MinBaseOut: 1, ServiceFeeLamports: 100_000,
		PlatformFeePubkey: solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
		ComputeUnitLimit:  200_000, ComputeUnitPrice: 1_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, compiled, err := solpkg.CompileV0Tx(user, solana.MustHashFromBase58("EkSnNWid2cvwEVnVx9aBqawnmiCNiDgp3gUdkDPTKN1N"), ixs, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(compiled.Transaction)
	tx, err := solana.TransactionFromBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	msg := tx.Message
	h := msg.Header
	t.Logf("header: sig=%d roSigned=%d roUnsigned=%d accounts=%d ixs=%d",
		h.NumRequiredSignatures, h.NumReadonlySignedAccounts, h.NumReadonlyUnsignedAccounts,
		len(msg.AccountKeys), len(msg.Instructions))
	for i, k := range msg.AccountKeys {
		signer := i < int(h.NumRequiredSignatures)
		var writable bool
		if signer {
			writable = i < int(h.NumRequiredSignatures-h.NumReadonlySignedAccounts)
		} else {
			roStart := len(msg.AccountKeys) - int(h.NumReadonlyUnsignedAccounts)
			writable = i < roStart
		}
		t.Logf("[%d] %s signer=%v writable=%v", i, k, signer, writable)
	}
	for i, ix := range msg.Instructions {
		acctIdxs := make([]int, len(ix.Accounts))
		for j, a := range ix.Accounts {
			acctIdxs[j] = int(a)
		}
		t.Logf("ix%d programIdx=%d acctIdxs=%v dataLen=%d", i, ix.ProgramIDIndex, acctIdxs, len(ix.Data))
	}

	// validate compact-u16 account list encoding manually
	for i, ix := range msg.Instructions {
		for j, acctIdx := range ix.Accounts {
			if int(acctIdx) >= len(msg.AccountKeys) {
				t.Fatalf("bad index ix%d acct%d -> %d", i, j, acctIdx)
			}
		}
		if int(ix.ProgramIDIndex) >= len(msg.AccountKeys) {
			t.Fatalf("bad program index ix%d -> %d", i, ix.ProgramIDIndex)
		}
	}
}
