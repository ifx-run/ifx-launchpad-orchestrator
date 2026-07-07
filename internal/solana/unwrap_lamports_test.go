package solana_test

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
)

func TestUnwrapLamportsInstruction_encoding(t *testing.T) {
	src := solana.NewWallet().PublicKey()
	dst := solana.NewWallet().PublicKey()
	owner := solana.NewWallet().PublicKey()
	amt := uint64(42)
	ix := solpkg.UnwrapLamportsInstruction(src, dst, owner, solana.TokenProgramID, &amt)
	data, err := ix.Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 10 || data[0] != 45 || data[1] != 1 || data[2] != 42 {
		t.Fatalf("data=%v", data)
	}
	allIx := solpkg.UnwrapLamportsInstruction(src, dst, owner, solana.TokenProgramID, nil)
	allData, err := allIx.Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(allData) != 2 || allData[0] != 45 || allData[1] != 0 {
		t.Fatalf("all data=%v", allData)
	}
}
