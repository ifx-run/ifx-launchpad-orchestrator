package solana_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/gagliardetto/solana-go"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func TestPumpBuyTx_base64RoundtripPreservesBytes(t *testing.T) {
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
	orig, _ := base64.StdEncoding.DecodeString(compiled.Transaction)
	tx, err := solana.TransactionFromBase64(compiled.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	remarshaled, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(orig, remarshaled) {
		t.Fatalf("roundtrip changed bytes: orig=%d remarshal=%d", len(orig), len(remarshaled))
	}
}
