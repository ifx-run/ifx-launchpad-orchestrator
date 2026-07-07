package orchestrator

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	ifxpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/ifx"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/route"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/util"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func (s *Service) buildPumpSwapTwoLeg(
	ctx context.Context,
	in QuoteInput,
	planned route.PlannedRoute,
	accounts map[solana.PublicKey]*rpc.Account,
	mintA, mintB, user, feePubkey solana.PublicKey,
	tier config.PriorityFeeTier,
) (*VariantsResult, error) {
	if len(planned.Legs) != 2 || planned.Legs[0].Kind != route.LegLaunchpad || planned.Legs[1].Kind != route.LegLaunchpad {
		return nil, fmt.Errorf("unexpected swap route legs")
	}
	if !ifxpkg.NeedsIfxForMultiLeg() {
		return nil, fmt.Errorf("swap requires Ifx orchestration")
	}

	curveA, globalA, mintAcctA, quoteKind, err := s.loadPumpCurveState(mintA, accounts)
	if err != nil {
		return nil, err
	}
	curveB, globalB, mintAcctB, quoteKindB, err := s.loadPumpCurveState(mintB, accounts)
	if err != nil {
		return nil, err
	}
	if quoteKind != quoteKindB {
		return nil, fmt.Errorf("swap tokens use different quote kinds")
	}
	wsolMint, err := solpkg.ParsePubkey(s.cfg.Quotes.WSOLMint)
	if err != nil {
		return nil, err
	}
	quoteMint, quoteTP, err := s.resolveQuoteToken(mintA, curveA, accounts, wsolMint)
	if err != nil {
		return nil, err
	}
	quoteMintB, _, err := s.resolveQuoteToken(mintB, curveB, accounts, wsolMint)
	if err != nil {
		return nil, err
	}
	if !quoteMint.Equals(quoteMintB) {
		return nil, fmt.Errorf("swap requires same pool quote mint")
	}

	baseDecA, err := pumpfun.MintDecimals(mintAcctA.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, baseDecA)
	if err != nil {
		return nil, err
	}

	gross := pumpfun.SellQuoteOut(globalA, curveA, inputRaw)
	minGross := util.MinOut(gross, in.SlippageBPS)
	serviceFee := util.ApplyBPS(minGross, s.cfg.ServiceFee.BPS)
	netQuote := minGross - serviceFee
	if serviceFee >= minGross {
		return nil, fmt.Errorf("output too small after service fee")
	}
	outB := pumpfun.BuyBaseOut(globalB, curveB, netQuote)
	minBaseB := util.MinOut(outB, in.SlippageBPS)

	buyAmounts, err := s.pumpBuyAmountsAfterRepay(
		in, planned.Legs, globalB, curveB, netQuote, in.SlippageBPS,
		estimatePumpSwapSponsoredATACreates(),
	)
	if err != nil {
		return nil, err
	}
	minBaseBSponsor := buyAmounts.MinBaseOutSponsor

	opFeeATA, err := operatorQuoteATA(feePubkey, quoteMint, quoteTP)
	if err != nil {
		return nil, err
	}
	quoteATA, err := userQuoteATA(user, quoteMint, accounts)
	if err != nil {
		return nil, err
	}

	sellParams := pumpfun.BuildParams{
		Curve:               curveA,
		BaseMint:            mintA,
		User:                user,
		BaseTokenProgram:    mintAcctA.Owner,
		QuoteMint:           quoteMint,
		QuoteTokenProgram:   quoteTP,
		CashbackEnabled:     curveA.IsCashbackCoin,
		BaseAmountIn:        inputRaw,
		MinQuoteOut:         minGross,
		PlatformFeePubkey:   feePubkey,
		PlatformFeeQuoteATA: opFeeATA,
		ComputeUnitLimit:    tier.ComputeUnitLimit,
		ComputeUnitPrice:    tier.MicroLamports,
	}
	sellTemplate, err := pumpfun.BuildSellCoreIx(sellParams, quoteKind)
	if err != nil {
		return nil, err
	}

	buyParams := pumpfun.BuildParams{
		Curve:               curveB,
		BaseMint:            mintB,
		User:                user,
		BaseTokenProgram:    mintAcctB.Owner,
		QuoteMint:           quoteMint,
		QuoteTokenProgram:   quoteTP,
		CashbackEnabled:     curveB.IsCashbackCoin,
		SpendableQuoteIn:    0,
		MinBaseOut:          minBaseB,
		PlatformFeePubkey:   feePubkey,
		PlatformFeeQuoteATA: opFeeATA,
		ComputeUnitLimit:    tier.ComputeUnitLimit,
		ComputeUnitPrice:    tier.MicroLamports,
	}
	buyTemplate, err := pumpfun.BuildBuyCoreIx(buyParams, quoteKindB)
	if err != nil {
		return nil, err
	}

	buyParamsSponsored := buyParams
	buyParamsSponsored.MinBaseOut = minBaseBSponsor
	buyTemplateSponsored, err := pumpfun.BuildBuyCoreIx(buyParamsSponsored, quoteKindB)
	if err != nil {
		return nil, err
	}

	ata := newATASetup()
	if err := ensurePumpBuyATAs(ata, user, mintB, mintAcctB.Owner, quoteMint, quoteTP, quoteKindB); err != nil {
		return nil, err
	}

	sellThenBuyParams := ifxpkg.SellThenBuyParams{
		QuoteKind:           quoteKind,
		SellTemplate:        sellTemplate,
		BuyTemplate:         buyTemplate,
		ServiceFeeBPS:       s.cfg.ServiceFee.BPS,
		User:                user,
		PlatformFeePubkey:   feePubkey,
		PlatformFeeQuoteATA: opFeeATA,
		QuoteATA:            quoteATA,
		QuoteMint:           quoteMint,
		QuoteTokenProgram:   quoteTP,
		QuoteDecimals:       quoteDecimals(s.cfg, quoteMint.String()),
	}
	sellThenBuyParamsSponsored := sellThenBuyParams
	sellThenBuyParamsSponsored.BuyTemplate = buyTemplateSponsored
	sponsoredWired := quoteKind == pumpfun.QuoteNativeSOL

	return s.compilePreflightVariants(ctx, in, user, tier, serviceFee, planned.Legs, ata.count(),
		func(mode VariantMode) ([]solana.Instruction, error) {
			if mode.Sponsored {
				return nil, nil
			}
			var pre []solana.Instruction
			if err := ata.appendTo(&pre, user); err != nil {
				return nil, err
			}
			return pre, nil
		},
		func(mode VariantMode) ([]solana.Instruction, error) {
			if mode.Sponsored && sponsoredWired {
				repay, err := s.sponsoredRepayParams(user)
				if err != nil {
					return nil, err
				}
				sponsor, err := s.sponsorPubkey()
				if err != nil {
					return nil, err
				}
				fixed := s.fixedSponsoredRepayFeesOnly(tier, mode)
				return ifxpkg.PlanPumpSellThenBuySponsored(
					s.cfg, sellThenBuyParamsSponsored, repay, fixed, sponsor, ata.ifxSpecs(),
				)
			}
			return ifxpkg.PlanPumpSellThenBuy(s.cfg, sellThenBuyParams)
		},
		sponsoredWired,
	)
}

func (s *Service) loadPumpCurveState(
	mint solana.PublicKey,
	accounts map[solana.PublicKey]*rpc.Account,
) (curve pumpfun.BondingCurve, global pumpfun.Global, mintAcct *rpc.Account, quoteKind pumpfun.QuoteKind, err error) {
	bcPK, err := pumpfun.BondingCurvePDAFromMint(mint)
	if err != nil {
		return
	}
	globalPK, err := pumpfun.GlobalPDA(s.cfg)
	if err != nil {
		return
	}
	bcAcct := accounts[bcPK]
	globalAcct := accounts[globalPK]
	mintAcct = accounts[mint]
	if bcAcct == nil || globalAcct == nil || mintAcct == nil {
		err = fmt.Errorf("snapshot missing pump accounts for %s", mint)
		return
	}
	curve, err = pumpfun.DecodeBondingCurve(bcAcct.Data.GetBinary())
	if err != nil {
		return
	}
	global, err = pumpfun.DecodeGlobal(globalAcct.Data.GetBinary())
	if err != nil {
		return
	}
	quoteKind = pumpfun.QuoteKindFor(s.cfg, curve)
	return
}
