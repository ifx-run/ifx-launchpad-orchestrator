package route_test

import (
	"testing"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
)

func TestIsSOLSettlementConvert(t *testing.T) {
	wsol := "So11111111111111111111111111111111111111112"
	usdc := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	cases := []struct {
		inMint, outMint, inSet, outSet string
		want                           bool
	}{
		{wsol, wsol, route.SettlementWSOLSPL, route.SettlementNativeSOL, true},
		{wsol, wsol, route.SettlementNativeSOL, route.SettlementWSOLSPL, true},
		{wsol, wsol, route.SettlementNativeSOL, route.SettlementNativeSOL, false},
		{wsol, usdc, route.SettlementNativeSOL, "spl", false},
	}

	for _, tc := range cases {
		got := route.IsSOLSettlementConvert(tc.inMint, tc.outMint, tc.inSet, tc.outSet, wsol)
		if got != tc.want {
			t.Fatalf("IsSOLSettlementConvert(%q,%q,%q,%q)=%v want %v", tc.inMint, tc.outMint, tc.inSet, tc.outSet, got, tc.want)
		}
	}
}

func TestSOLSettlementUnwrap(t *testing.T) {
	if !route.SOLSettlementUnwrap(route.SettlementWSOLSPL, route.SettlementNativeSOL) {
		t.Fatal("expected unwrap")
	}
	if route.SOLSettlementUnwrap(route.SettlementNativeSOL, route.SettlementWSOLSPL) {
		t.Fatal("wrap is not unwrap")
	}
}
