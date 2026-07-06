package solana_test

import (
	"testing"

	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
)

func TestCompileV0TxRoundtrip(t *testing.T) {
	payer := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	blockhash := solana.MustHashFromBase58("EkSnNWid2cvwEVnVx9aBqawnmiCNiDgp3gUdkDPTKN1N")

	ixs := []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(500_000).Build(),
		system.NewTransferInstruction(1000, payer, payer).Build(),
	}

	_, compiled, err := solpkg.CompileV0Tx(payer, blockhash, ixs, nil)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := solana.TransactionFromBase64(compiled.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != compiled.TransactionSize {
		t.Fatalf("size mismatch %d vs %d", len(raw), compiled.TransactionSize)
	}
	ins, err := solpkg.InspectTransactionBase64(compiled.Transaction, compiled.TransactionSize)
	if err != nil {
		t.Fatal(err)
	}
	if ins.NumInstructions != 2 {
		t.Fatalf("instructions=%d", ins.NumInstructions)
	}
	if len(ins.Instructions) != 2 {
		t.Fatalf("inspection slice len=%d", len(ins.Instructions))
	}
	if ins.Format != "v0" {
		t.Fatalf("format=%s want v0", ins.Format)
	}
}
