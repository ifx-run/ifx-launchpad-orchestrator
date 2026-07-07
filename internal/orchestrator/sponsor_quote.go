package orchestrator

import (
	"fmt"

	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/route"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/util"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

// pumpBuyAmounts holds pump leg sizing for selfFunded vs sponsored (post-repay) paths.
type pumpBuyAmounts struct {
	GrossPumpIn       uint64
	MinBaseOutGross   uint64
	SponsoredPumpIn   uint64
	MinBaseOutSponsor uint64
	RepayDeducted     uint64
}

func (s *Service) shouldDeductSponsorRepayInQuote(in QuoteInput, legs []route.Leg) bool {
	if !s.cfg.Sponsor.Enabled {
		return false
	}
	return route.SponsoredRepayEligible(
		in.InputMint,
		in.OutputMint,
		in.InputSettlement,
		in.OutputSettlement,
		s.cfg.Quotes.WSOLMint,
		legs,
	)
}

// quoteSponsoredRepayDeduction is a conservative repay estimate for quote / minBaseOut sizing.
func (s *Service) quoteSponsoredRepayDeduction(in QuoteInput, ataCreates int) uint64 {
	tier := s.cfg.Tier(in.PriorityTier)
	tip := jitoTipLamports(s.cfg, s.cfg.Jito.Enabled)
	return EstimateRepayLamports(s.cfg, tier, 2, ataCreates, tip)
}

func subtractRepayFromPumpIn(gross, repay uint64) (uint64, error) {
	if repay == 0 {
		return gross, nil
	}
	if repay >= gross {
		return 0, fmt.Errorf("insufficient SOL/WSOL (%d) for sponsor repay (%d)", gross, repay)
	}
	return gross - repay, nil
}

func (s *Service) pumpBuyAmountsAfterRepay(
	in QuoteInput,
	legs []route.Leg,
	global pumpfun.Global,
	curve pumpfun.BondingCurve,
	grossPumpIn uint64,
	slippageBPS int,
	ataCreates int,
) (pumpBuyAmounts, error) {
	outGross := pumpfun.BuyBaseOut(global, curve, grossPumpIn)
	amt := pumpBuyAmounts{
		GrossPumpIn:     grossPumpIn,
		MinBaseOutGross: util.MinOut(outGross, slippageBPS),
		SponsoredPumpIn: grossPumpIn,
		RepayDeducted:   0,
	}
	amt.MinBaseOutSponsor = amt.MinBaseOutGross

	if !s.shouldDeductSponsorRepayInQuote(in, legs) {
		return amt, nil
	}

	repay := s.quoteSponsoredRepayDeduction(in, ataCreates)
	pumpIn, err := subtractRepayFromPumpIn(grossPumpIn, repay)
	if err != nil {
		return pumpBuyAmounts{}, err
	}
	outSp := pumpfun.BuyBaseOut(global, curve, pumpIn)
	amt.SponsoredPumpIn = pumpIn
	amt.MinBaseOutSponsor = util.MinOut(outSp, slippageBPS)
	amt.RepayDeducted = repay
	return amt, nil
}

// estimatePumpSellTwoLegATACreates matches typical sponsor ATA creates in sell→bridge Ifx.
func estimatePumpSellTwoLegATACreates(quoteKind pumpfun.QuoteKind) int {
	n := 1 // bridge output mint (e.g. USDT)
	if quoteKind == pumpfun.QuoteNativeSOL {
		n++ // WSOL ATA for wrap before bridge
	}
	return n
}

// sellBridgeAmounts holds bridge leg sizing for selfFunded vs sponsored (post-repay) sell paths.
type sellBridgeAmounts struct {
	GrossBridgeIn     uint64
	SponsoredBridgeIn uint64
	BridgeOutQuoted   uint64
	MinOutGross       uint64
	MinOutSponsor     uint64
	RepayDeducted     uint64
}

func scaleBridgeOut(quotedOut, grossIn, netIn uint64) uint64 {
	if grossIn == 0 || netIn == 0 {
		return 0
	}
	return quotedOut * netIn / grossIn
}

func (s *Service) sellBridgeAmountsAfterRepay(
	in QuoteInput,
	legs []route.Leg,
	grossBridgeIn uint64,
	bridgeOutQuoted uint64,
	slippageBPS int,
	ataCreates int,
) (sellBridgeAmounts, error) {
	amt := sellBridgeAmounts{
		GrossBridgeIn:     grossBridgeIn,
		SponsoredBridgeIn: grossBridgeIn,
		BridgeOutQuoted:   bridgeOutQuoted,
		MinOutGross:       util.MinOut(bridgeOutQuoted, slippageBPS),
		RepayDeducted:     0,
	}
	amt.MinOutSponsor = amt.MinOutGross

	if !s.shouldDeductSponsorRepayInQuote(in, legs) {
		return amt, nil
	}

	repay := s.quoteSponsoredRepayDeduction(in, ataCreates)
	netIn, err := subtractRepayFromPumpIn(grossBridgeIn, repay)
	if err != nil {
		return sellBridgeAmounts{}, err
	}
	scaledOut := scaleBridgeOut(bridgeOutQuoted, grossBridgeIn, netIn)
	amt.SponsoredBridgeIn = netIn
	amt.MinOutSponsor = util.MinOut(scaledOut, slippageBPS)
	amt.RepayDeducted = repay
	return amt, nil
}

// estimatePumpSwapSponsoredATACreates matches sponsor ATA create for output base mint on A→B swap.
func estimatePumpSwapSponsoredATACreates() int { return 1 }

func estimatePumpBuyTwoLegATACreates(wrapBridgeSOL bool) int {
	n := 3 // base + pay mint + WSOL measure ATA
	if wrapBridgeSOL {
		n++ // user WSOL wrap ATA (may overlap; stay conservative)
	}
	return n
}
