package orchestrator

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
)

func TestJitoTipInstruction_zeroLamportsStillBuilds(t *testing.T) {
	cfg := &config.Config{
		Jito: config.JitoConfig{
			Enabled:     true,
			TipAccount:  "96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5",
			TipLamports: 0,
		},
	}
	payer := solana.MustPublicKeyFromBase58("AJ9WRJAfdXs5a3vbqvvLam2ueay2FTjiChPSzNWWbxAH")
	ix, err := JitoTipInstruction(cfg, payer, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !ix.ProgramID().Equals(solana.SystemProgramID) {
		t.Fatalf("expected system program, got %s", ix.ProgramID())
	}
}

func TestPrependComputeBudget_mevAddsTipTransfer(t *testing.T) {
	tier := config.PriorityFeeTier{ComputeUnitLimit: 500_000, MicroLamports: 10_000}
	core := []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(999).Build(), // stripped
		solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		}, []byte{0}),
	}
	ixs := prependComputeBudget(tier, core)
	cfg := &config.Config{
		Jito: config.JitoConfig{
			Enabled:     true,
			TipAccount:  "96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5",
			TipLamports: 1000,
		},
	}
	payer := solana.MustPublicKeyFromBase58("AJ9WRJAfdXs5a3vbqvvLam2ueay2FTjiChPSzNWWbxAH")
	tipIx, err := JitoTipInstruction(cfg, payer, cfg.Jito.TipLamports)
	if err != nil {
		t.Fatal(err)
	}
	mevIxs := append(ixs, tipIx)
	if countComputeBudget(mevIxs) != 2 {
		t.Fatalf("expected 2 compute budget ixs, got %d", countComputeBudget(mevIxs))
	}
	if len(mevIxs) != len(ixs)+1 {
		t.Fatalf("mev should add one tip ix")
	}
}
