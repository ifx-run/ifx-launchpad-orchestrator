package pumpfun_test

import (
	"testing"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/util"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func TestQuoteSell_serviceFeeOnMinGross(t *testing.T) {
	global := pumpfun.Global{FeeBasisPoints: 100, CreatorFeeBasisPoints: 0}
	curve := pumpfun.BondingCurve{
		VirtualTokenReserves: 1_073_000_000_000_000,
		VirtualSolReserves:   30_000_000_000,
		RealTokenReserves:    793_100_000_000_000,
		TokenTotalSupply:     1_000_000_000_000_000,
		Creator:              pumpfun.DefaultPubkey(),
	}
	gross := pumpfun.SellQuoteOut(global, curve, 1_000_000_000_000)
	minGross := util.MinOut(gross, 100) // 1% slippage
	fee := util.ApplyBPS(minGross, 50)  // 0.5% platform fee

	if fee >= minGross {
		t.Fatal("fee should be less than min gross")
	}
	if fee > util.ApplyBPS(gross, 50) {
		t.Fatal("conservative fee must not exceed fee on expected gross")
	}
}
