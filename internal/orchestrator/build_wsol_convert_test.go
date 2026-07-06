package orchestrator

import (
	"testing"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
)

func hasComputeBudgetIxs(ixs []solana.Instruction) bool {
	for _, ix := range ixs {
		if ix.ProgramID().Equals(computebudget.ProgramID) {
			return true
		}
	}
	return false
}

func TestPrependComputeBudget_skipsWhenTierDisabled(t *testing.T) {
	embedded := []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(500_000).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(10_000).Build(),
	}
	out := prependComputeBudget(settlementNoPriorityTier(), embedded)
	if hasComputeBudgetIxs(out) {
		t.Fatal("disabled tier must not emit compute-budget instructions")
	}
}

func TestSettlementUnwrapInstructions_noComputeBudget(t *testing.T) {
	owner := solana.NewWallet().PublicKey()
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	for _, mode := range []solpkg.WSOLUnwrapMode{
		solpkg.WSOLUnwrapPartial,
		solpkg.WSOLUnwrapClose,
		solpkg.WSOLUnwrapLamportsAll,
	} {
		ixs, err := solpkg.UnwrapWSOLInstructions(owner, wsol, solana.TokenProgramID, 10_000_000, mode)
		if err != nil {
			t.Fatalf("mode %v: %v", mode, err)
		}
		if hasComputeBudgetIxs(ixs) {
			t.Fatalf("mode %v: unexpected compute budget ix", mode)
		}
	}
}

func TestSettlementUnwrapCompiledTx_noComputeBudget(t *testing.T) {
	owner := solana.NewWallet().PublicKey()
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	ixs, err := solpkg.UnwrapWSOLInstructions(owner, wsol, solana.TokenProgramID, 10_000_000, solpkg.WSOLUnwrapPartial)
	if err != nil {
		t.Fatal(err)
	}
	ixs = stripComputeBudgetIxs(ixs)
	tx, _, err := solpkg.CompileV0Tx(owner, solana.Hash{}, ixs, nil)
	if err != nil {
		t.Fatal(err)
	}
	insp := solpkg.InspectTransaction(tx, 0)
	for _, ix := range insp.Instructions {
		if ix.ProgramLabel == "Compute Budget" {
			t.Fatalf("instruction %d is compute budget", ix.Index)
		}
	}
}

func TestBuildSponsoredUnwrapInstructions_appendsSystemTransfer(t *testing.T) {
	cfg := &config.Config{
		Sponsor: config.SponsorConfig{
			Enabled:     true,
			Pubkey:      "BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P",
			RepayPubkey: "CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM",
		},
	}
	svc := &Service{cfg: cfg}
	user := solana.MustPublicKeyFromBase58(cfg.Sponsor.Pubkey)
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")

	ixs, err := svc.buildSponsoredUnwrapInstructions(user, wsol, 10_000_000, solpkg.WSOLUnwrapPartial, 11_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) != 3 {
		t.Fatalf("want sync+unwrap+repay, got %d", len(ixs))
	}
	if !ixs[2].ProgramID().Equals(system.ProgramID) {
		t.Fatalf("repay program=%s", ixs[2].ProgramID())
	}
	if hasComputeBudgetIxs(ixs) {
		t.Fatal("sponsored unwrap must not include compute-budget instructions")
	}
}

func TestBuildSponsoredUnwrapInstructions_rejectsSmallAmount(t *testing.T) {
	cfg := &config.Config{
		Sponsor: config.SponsorConfig{
			Enabled: true,
			Pubkey:  "BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P",
		},
	}
	svc := &Service{cfg: cfg}
	user := solana.MustPublicKeyFromBase58(cfg.Sponsor.Pubkey)
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")

	_, err := svc.buildSponsoredUnwrapInstructions(user, wsol, 5_000, solpkg.WSOLUnwrapPartial, 11_000)
	if err == nil {
		t.Fatal("expected insufficient unwrap error")
	}
}
