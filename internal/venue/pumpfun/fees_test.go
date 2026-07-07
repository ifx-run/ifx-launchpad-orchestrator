package pumpfun_test

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func TestPickFeeRecipient_mayhemUsesReserved(t *testing.T) {
	normal := pumpfun.PickFeeRecipient(false)
	reserved := pumpfun.PickFeeRecipient(true)
	if normal.Equals(reserved) {
		t.Fatal("mayhem fee recipient should differ from normal")
	}
	expected := solana.MustPublicKeyFromBase58("GesfTA3X2arioaHp8bbKdjG9vJtskViWACZoYvxp4twS")
	if !reserved.Equals(expected) {
		t.Fatalf("reserved fee recipient=%s want %s", reserved, expected)
	}
}
