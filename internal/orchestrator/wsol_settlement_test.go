package orchestrator

import "testing"

func TestNormalizeSettlement_wsolDefaultsNative(t *testing.T) {
	wsol := "So11111111111111111111111111111111111111112"
	if got := NormalizeSettlement(wsol, "", wsol); got != SettlementNativeSOL {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeSettlement(wsol, SettlementWSOLSPL, wsol); got != SettlementWSOLSPL {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeSettlement_stableDefaultSpl(t *testing.T) {
	usdc := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	wsol := "So11111111111111111111111111111111111111112"
	if got := NormalizeSettlement(usdc, "", wsol); got != SettlementSPL {
		t.Fatalf("got %q", got)
	}
}
