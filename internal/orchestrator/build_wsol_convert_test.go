package orchestrator

import (
	"testing"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

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
