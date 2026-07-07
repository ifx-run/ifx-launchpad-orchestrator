package solana_test

import (
	"encoding/base64"
	"testing"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func TestCompileV0Tx_unsignedHasPlaceholderSignature(t *testing.T) {
	payer := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	bh := solana.MustHashFromBase58("EkSnNWid2cvwEVnVx9aBqawnmiCNiDgp3gUdkDPTKN1N")
	ixs := []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(500_000).Build(),
		system.NewTransferInstruction(1000, payer, payer).Build(),
	}
	_, compiled, err := solpkg.CompileV0Tx(payer, bh, ixs, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(compiled.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	decoder := bin.NewBinDecoder(raw)
	numSigs, err := decoder.ReadCompactU16()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := solana.TransactionFromBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	required := int(tx.Message.Header.NumRequiredSignatures)
	t.Logf("wire sigs=%d required=%d tx.Signatures=%d", numSigs, required, len(tx.Signatures))
	if numSigs != required {
		t.Fatalf("unsigned tx must have %d placeholder signatures on wire, got %d", required, numSigs)
	}
}

func TestCompileV0Tx_pumpBuyPlaceholderSignature(t *testing.T) {
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
	decoder := bin.NewBinDecoder(raw)
	numSigs, _ := decoder.ReadCompactU16()
	tx, _ := solana.TransactionFromBytes(raw)
	if numSigs != int(tx.Message.Header.NumRequiredSignatures) {
		t.Fatalf("wire sigs=%d required=%d", numSigs, tx.Message.Header.NumRequiredSignatures)
	}
}
