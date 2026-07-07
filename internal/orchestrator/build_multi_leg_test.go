package orchestrator

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
)

func TestStripComputeBudgetIxs_removesOnlyComputeBudget(t *testing.T) {
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	ixs := []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(500_000).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(10_000).Build(),
		solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
			{PublicKey: user, IsSigner: true, IsWritable: true},
		}, []byte{0}),
	}
	out := stripComputeBudgetIxs(ixs)
	if len(out) != 1 {
		t.Fatalf("expected 1 instruction after strip, got %d", len(out))
	}
	if out[0].ProgramID().Equals(computebudget.ProgramID) {
		t.Fatal("expected non-compute-budget instruction to remain")
	}
}

func countComputeBudget(ixs []solana.Instruction) int {
	n := 0
	for _, ix := range ixs {
		if ix.ProgramID().Equals(computebudget.ProgramID) {
			n++
		}
	}
	return n
}

func TestPrependComputeBudget_dedupesEmbedded(t *testing.T) {
	tier := config.PriorityFeeTier{
		ComputeUnitLimit: 500_000,
		MicroLamports:    10_000,
	}
	embedded := []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(500_000).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(10_000).Build(),
		solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
			{PublicKey: solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P"), IsSigner: true, IsWritable: true},
		}, []byte{0}),
	}
	combined := prependComputeBudget(tier, embedded)
	if got := countComputeBudget(combined); got != 2 {
		t.Fatalf("expected exactly 2 compute budget instructions, got %d", got)
	}
	if len(combined) != 3 {
		t.Fatalf("expected 3 instructions total, got %d", len(combined))
	}
}
