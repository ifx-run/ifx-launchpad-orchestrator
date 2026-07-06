package route

import "testing"

func TestPlanSwapAB_twoLaunchpadLegs(t *testing.T) {
	wsol := "So11111111111111111111111111111111111111112"
	tokenA := "TokenMintA111"
	tokenB := "TokenMintB222"
	isQuote := func(m string) bool { return m == wsol }

	r := PlanLaunchpadRoute(PairSwapLaunchpad, tokenA, tokenB, wsol, isQuote)
	if r.HopCount != 2 {
		t.Fatalf("hop=%d", r.HopCount)
	}
	if r.Legs[0].Kind != LegLaunchpad || r.Legs[1].Kind != LegLaunchpad {
		t.Fatalf("legs=%+v", r.Legs)
	}
	if HasBridgeLeg(r) {
		t.Fatal("swap AB should not include bridge")
	}
}

func TestPlanBuy_twoLeg(t *testing.T) {
	wsol := "So11111111111111111111111111111111111111112"
	usdc := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	token := "TokenMint111"
	isQuote := func(m string) bool { return m == wsol || m == usdc }

	r := PlanLaunchpadRoute(PairBuyLaunchpad, usdc, token, wsol, isQuote)
	if r.HopCount != 2 {
		t.Fatalf("hop=%d", r.HopCount)
	}
	if r.Legs[0].Kind != LegQuoteBridge || r.Legs[1].Kind != LegLaunchpad {
		t.Fatalf("legs=%+v", r.Legs)
	}
}

func TestPlanSell_oneLeg(t *testing.T) {
	wsol := "So11111111111111111111111111111111111111112"
	token := "TokenMint111"
	isQuote := func(m string) bool { return m == wsol }

	r := PlanLaunchpadRoute(PairSellLaunchpad, token, wsol, wsol, isQuote)
	if r.HopCount != 1 {
		t.Fatalf("hop=%d", r.HopCount)
	}
}

func TestSponsoredRepayEligible_userAssets(t *testing.T) {
	wsol := "So11111111111111111111111111111111111111112"
	usdt := "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
	token := "AooGdzgK4PvsbdNoZos4NtStWdkX4z2EpsMp6pYwR9JL"
	legs := []Leg{
		{Kind: LegQuoteBridge, InputMint: usdt, OutputMint: wsol},
		{Kind: LegLaunchpad, InputMint: wsol, OutputMint: token},
	}

	if !SponsoredRepayEligible(wsol, token, "native_sol", "spl", wsol, nil) {
		t.Fatal("SOL buy should be sponsored-repay eligible")
	}
	if !SponsoredRepayEligible(token, wsol, "spl", "native_sol", wsol, nil) {
		t.Fatal("sell to native SOL should be sponsored-repay eligible")
	}
	if !SponsoredRepayEligible(usdt, token, "spl", "spl", wsol, legs) {
		t.Fatal("USDT bridge buy should be sponsored-repay eligible")
	}
	if SponsoredSwapEligible(usdt, token, "spl", "spl", wsol) {
		t.Fatal("user assets alone should not qualify")
	}
}
