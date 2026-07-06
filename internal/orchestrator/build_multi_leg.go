package orchestrator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/bridge"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	ifxpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/ifx"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/util"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

func (s *Service) buildPumpMultiLeg(
	ctx context.Context,
	in QuoteInput,
	pairClass route.PairClass,
	planned route.PlannedRoute,
	bridgePool *bridge.DiscoveredPool,
	accounts map[solana.PublicKey]*rpc.Account,
	detection *venue.Detection,
	baseMint solana.PublicKey,
) (*VariantsResult, error) {
	if detection.Venue != venue.IDPumpfun {
		return nil, fmt.Errorf("multi-leg build only wired for pumpfun")
	}
	user, err := solpkg.ParsePubkey(in.UserPubkey)
	if err != nil {
		return nil, err
	}
	feePubkey, err := solpkg.ParsePubkey(s.cfg.ServiceFee.Pubkey)
	if err != nil {
		return nil, err
	}
	tier := s.cfg.Tier(in.PriorityTier)

	switch pairClass {
	case route.PairBuyLaunchpad:
		return s.buildPumpBuyTwoLeg(ctx, in, planned, bridgePool, accounts, baseMint, user, feePubkey, tier)
	case route.PairSellLaunchpad:
		return s.buildPumpSellTwoLeg(ctx, in, planned, bridgePool, accounts, baseMint, user, feePubkey, tier)
	default:
		return nil, fmt.Errorf("multi-leg build unsupported for pair class %s", pairClass)
	}
}

func (s *Service) buildPumpBuyTwoLeg(
	ctx context.Context,
	in QuoteInput,
	planned route.PlannedRoute,
	bridgePool *bridge.DiscoveredPool,
	accounts map[solana.PublicKey]*rpc.Account,
	baseMint, user, feePubkey solana.PublicKey,
	tier config.PriorityFeeTier,
) (*VariantsResult, error) {
	if len(planned.Legs) != 2 || planned.Legs[0].Kind != route.LegQuoteBridge || planned.Legs[1].Kind != route.LegLaunchpad {
		return nil, fmt.Errorf("unexpected buy route legs")
	}
	if !ifxpkg.NeedsIfxForMultiLeg() {
		return nil, fmt.Errorf("multi-leg buy requires Ifx orchestration")
	}
	bcPK, globalPK, curve, global, mintAcct, quoteKind, err := s.loadPumpBuildState(baseMint, accounts)
	if err != nil {
		return nil, err
	}
	_ = bcPK
	_ = globalPK

	wsolMint, err := solpkg.ParsePubkey(s.cfg.Quotes.WSOLMint)
	if err != nil {
		return nil, err
	}

	bridgeInMint, err := solpkg.ParsePubkey(planned.Legs[0].InputMint)
	if err != nil {
		return nil, err
	}
	bridgeOutMint, err := solpkg.ParsePubkey(planned.Legs[0].OutputMint)
	if err != nil {
		return nil, err
	}
	bridgeInDec := quoteDecimals(s.cfg, planned.Legs[0].InputMint)
	inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, bridgeInDec)
	if err != nil {
		return nil, err
	}
	serviceFee := util.ApplyBPS(inputRaw, s.cfg.ServiceFee.BPS)
	if serviceFee >= inputRaw {
		return nil, fmt.Errorf("input too small after service fee")
	}
	bridgeIn := inputRaw - serviceFee

	bridgeOut, err := strconv.ParseUint(bridgePool.OutAmount, 10, 64)
	if err != nil {
		return nil, err
	}

	quoteMint, quoteTP, err := s.resolveQuoteToken(baseMint, curve, accounts, wsolMint)
	if err != nil {
		return nil, err
	}
	opFeeATA, err := operatorQuoteATA(feePubkey, quoteMint, quoteTP)
	if err != nil {
		return nil, err
	}

	measureATA, err := userQuoteATA(user, bridgeOutMint, accounts)
	if err != nil {
		return nil, err
	}

	ata := newATASetup()
	if err := ensurePumpBuyATAs(ata, user, baseMint, mintAcct.Owner, quoteMint, quoteTP, quoteKind); err != nil {
		return nil, err
	}
	bridgeSwap, err := s.bridgeLegInstructions(ata, bridgePool, accounts, user, bridgeInMint, bridgeIn, bridgeOut, in.SlippageBPS, false)
	if err != nil {
		return nil, err
	}
	wrapBridgeSOL := shouldWrapSOLForBridge(in, bridgeInMint, wsolMint, bridgeIn)
	if wrapBridgeSOL {
		if err := ata.ensure(user, user, wsolMint, solana.TokenProgramID); err != nil {
			return nil, err
		}
	}

	amounts, err := s.pumpBuyAmountsAfterRepay(
		in, planned.Legs, global, curve, bridgeOut, in.SlippageBPS, ata.count(),
	)
	if err != nil {
		return nil, err
	}

	paramsUser := pumpfun.BuildParams{
		Curve:               curve,
		BaseMint:            baseMint,
		User:                user,
		BaseTokenProgram:    mintAcct.Owner,
		QuoteMint:           quoteMint,
		QuoteTokenProgram:   quoteTP,
		CashbackEnabled:     curve.IsCashbackCoin,
		SpendableQuoteIn:    0,
		MinBaseOut:          amounts.MinBaseOutGross,
		PlatformFeePubkey:   feePubkey,
		PlatformFeeQuoteATA: opFeeATA,
		ComputeUnitLimit:    tier.ComputeUnitLimit,
		ComputeUnitPrice:    tier.MicroLamports,
	}
	paramsSponsored := paramsUser
	paramsSponsored.MinBaseOut = amounts.MinBaseOutSponsor

	buyTemplateUser, err := pumpfun.BuildBuyCoreIx(paramsUser, quoteKind)
	if err != nil {
		return nil, err
	}
	buyTemplateSponsored, err := pumpfun.BuildBuyCoreIx(paramsSponsored, quoteKind)
	if err != nil {
		return nil, err
	}

	var commonPreflight []solana.Instruction
	if err := s.appendPayAssetFeeTransfer(&commonPreflight, bridgeInMint, user, feePubkey, accounts, serviceFee); err != nil {
		return nil, err
	}
	if wrapBridgeSOL {
		if err := appendWrapSOLDeposit(&commonPreflight, user, user, wsolMint, solana.TokenProgramID, bridgeIn); err != nil {
			return nil, err
		}
	}

	var unwrap *solana.Instruction
	if quoteKind == pumpfun.QuoteNativeSOL {
		unwrapIx, err := solpkg.CloseWSOLATA(user, wsolMint, solana.TokenProgramID)
		if err != nil {
			return nil, err
		}
		unwrap = &unwrapIx
	}

	bridgeParamsUser := ifxpkg.BridgeThenPumpBuyParams{
		BridgeSwap:      bridgeSwap,
		MeasureQuoteATA: measureATA,
		BuyTemplate:     buyTemplateUser,
		UnwrapWSOL:      unwrap,
	}
	bridgeParamsSponsored := bridgeParamsUser
	bridgeParamsSponsored.BuyTemplate = buyTemplateSponsored

	if quoteKind != pumpfun.QuoteNativeSOL || unwrap == nil {
		var ixs []solana.Instruction
		if err := ata.appendTo(&ixs, user); err != nil {
			return nil, err
		}
		ixs = append(ixs, commonPreflight...)
		ifxIxs, err := ifxpkg.PlanBridgeThenPumpBuy(s.cfg, bridgeParamsUser)
		if err != nil {
			return nil, err
		}
		ixs = append(ixs, ifxIxs...)
		return s.compileVariantsFromIXs(ctx, in, user, tier, ixs, serviceFee, planned.Legs, false, ata.count())
	}

	return s.compilePreflightVariants(ctx, in, user, tier, serviceFee, planned.Legs, ata.count(),
		func(mode VariantMode) ([]solana.Instruction, error) {
			var preflight []solana.Instruction
			if !mode.Sponsored {
				payer, err := s.ataPayerForMode(mode, user)
				if err != nil {
					return nil, err
				}
				if err := ata.appendTo(&preflight, payer); err != nil {
					return nil, err
				}
			}
			preflight = append(preflight, commonPreflight...)
			return preflight, nil
		},
		func(mode VariantMode) ([]solana.Instruction, error) {
			if mode.Sponsored {
				repay, err := s.sponsoredRepayParams(user)
				if err != nil {
					return nil, err
				}
				sponsor, err := s.sponsorPubkey()
				if err != nil {
					return nil, err
				}
				fixed := s.fixedSponsoredRepayFeesOnly(tier, mode)
				return ifxpkg.PlanBridgeThenPumpBuySponsored(s.cfg, bridgeParamsSponsored, repay, fixed, sponsor, ata.ifxSpecs())
			}
			return ifxpkg.PlanBridgeThenPumpBuy(s.cfg, bridgeParamsUser)
		}, true)
}

func (s *Service) buildPumpSellTwoLeg(
	ctx context.Context,
	in QuoteInput,
	planned route.PlannedRoute,
	bridgePool *bridge.DiscoveredPool,
	accounts map[solana.PublicKey]*rpc.Account,
	baseMint, user, feePubkey solana.PublicKey,
	tier config.PriorityFeeTier,
) (*VariantsResult, error) {
	if len(planned.Legs) != 2 || planned.Legs[0].Kind != route.LegLaunchpad || planned.Legs[1].Kind != route.LegQuoteBridge {
		return nil, fmt.Errorf("unexpected sell route legs")
	}
	if !ifxpkg.NeedsIfxForMultiLeg() {
		return nil, fmt.Errorf("multi-leg sell requires Ifx orchestration")
	}
	_, _, curve, global, mintAcct, quoteKind, err := s.loadPumpBuildState(baseMint, accounts)
	if err != nil {
		return nil, err
	}
	wsolMint, err := solpkg.ParsePubkey(s.cfg.Quotes.WSOLMint)
	if err != nil {
		return nil, err
	}
	baseDecimals, err := pumpfun.MintDecimals(mintAcct.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, baseDecimals)
	if err != nil {
		return nil, err
	}
	gross := pumpfun.SellQuoteOut(global, curve, inputRaw)
	minGross := util.MinOut(gross, in.SlippageBPS)

	quoteMint, quoteTP, err := s.resolveQuoteToken(baseMint, curve, accounts, wsolMint)
	if err != nil {
		return nil, err
	}
	opFeeATA, err := operatorQuoteATA(feePubkey, quoteMint, quoteTP)
	if err != nil {
		return nil, err
	}
	quoteATA, err := userQuoteATA(user, quoteMint, accounts)
	if err != nil {
		return nil, err
	}
	wsolATA, err := userQuoteATA(user, wsolMint, accounts)
	if err != nil {
		return nil, err
	}

	params := pumpfun.BuildParams{
		Curve:               curve,
		BaseMint:            baseMint,
		User:                user,
		BaseTokenProgram:    mintAcct.Owner,
		QuoteMint:           quoteMint,
		QuoteTokenProgram:   quoteTP,
		CashbackEnabled:     curve.IsCashbackCoin,
		BaseAmountIn:        inputRaw,
		MinQuoteOut:         minGross,
		PlatformFeePubkey:   feePubkey,
		PlatformFeeQuoteATA: opFeeATA,
		ComputeUnitLimit:    tier.ComputeUnitLimit,
		ComputeUnitPrice:    tier.MicroLamports,
	}
	sellTemplate, err := pumpfun.BuildSellCoreIx(params, quoteKind)
	if err != nil {
		return nil, err
	}

	bridgeOut, err := strconv.ParseUint(bridgePool.OutAmount, 10, 64)
	if err != nil {
		return nil, err
	}
	bridgeInMint, err := solpkg.ParsePubkey(planned.Legs[1].InputMint)
	if err != nil {
		return nil, err
	}
	serviceFee := util.ApplyBPS(minGross, s.cfg.ServiceFee.BPS)
	ata := newATASetup()
	bridgeTemplateUser, err := s.bridgeLegInstructions(ata, bridgePool, accounts, user,
		bridgeInMint, 0, bridgeOut, in.SlippageBPS, quoteKind == pumpfun.QuoteNativeSOL)
	if err != nil {
		return nil, err
	}
	grossBridgeIn := minGross - serviceFee
	bridgeAmounts, err := s.sellBridgeAmountsAfterRepay(
		in, planned.Legs, grossBridgeIn, bridgeOut, in.SlippageBPS,
		estimatePumpSellTwoLegATACreates(quoteKind),
	)
	if err != nil {
		return nil, err
	}
	bridgeTemplateSponsored, err := s.bridgeLegInstructionsWithMinOut(ata, bridgePool, accounts, user,
		bridgeInMint, 0, bridgeOut, in.SlippageBPS, bridgeAmounts.MinOutSponsor, quoteKind == pumpfun.QuoteNativeSOL)
	if err != nil {
		return nil, err
	}
	if quoteKind == pumpfun.QuoteNativeSOL {
		if err := ata.ensure(user, user, wsolMint, solana.TokenProgramID); err != nil {
			return nil, err
		}
	}

	sellBridgeParamsUser := ifxpkg.SellThenBridgeParams{
		QuoteKind:            quoteKind,
		SellTemplate:         sellTemplate,
		BridgeTemplate:       bridgeTemplateUser,
		BridgeAmountInOffset: bridgePool.PoolType.AmountInDataOffset(),
		ServiceFeeBPS:        s.cfg.ServiceFee.BPS,
		User:                 user,
		PlatformFeePubkey:    feePubkey,
		PlatformFeeQuoteATA:  opFeeATA,
		QuoteATA:             quoteATA,
		QuoteMint:            quoteMint,
		QuoteTokenProgram:    quoteTP,
		QuoteDecimals:        quoteDecimals(s.cfg, quoteMint.String()),
		WSOLATA:              wsolATA,
		WrapBeforeBridge:     quoteKind == pumpfun.QuoteNativeSOL,
	}
	sellBridgeParamsSponsored := sellBridgeParamsUser
	sellBridgeParamsSponsored.BridgeTemplate = bridgeTemplateSponsored
	sponsoredWired := quoteKind == pumpfun.QuoteNativeSOL

	bridgeOutMint, err := solpkg.ParsePubkey(planned.Legs[1].OutputMint)
	if err != nil {
		return nil, err
	}

	return s.compileAllVariants(ctx, in, func(mode VariantMode) (TxPlan, error) {
		var ifxIxs []solana.Instruction
		if mode.Sponsored && sponsoredWired {
			repay, err := s.sponsoredRepayParams(user)
			if err != nil {
				return TxPlan{}, err
			}
			sponsor, err := s.sponsorPubkey()
			if err != nil {
				return TxPlan{}, err
			}
			fixed := s.fixedSponsoredRepayFeesOnly(tier, mode)
			ifxIxs, err = ifxpkg.PlanPumpSellThenBridgeSponsored(s.cfg, sellBridgeParamsSponsored, repay, fixed, sponsor, ata.ifxSpecs())
		} else {
			ifxIxs, err = ifxpkg.PlanPumpSellThenBridge(s.cfg, sellBridgeParamsUser)
		}
		if err != nil {
			return TxPlan{}, err
		}
		var ixs []solana.Instruction
		if !mode.Sponsored {
			if err := ata.appendTo(&ixs, user); err != nil {
				return TxPlan{}, err
			}
		}
		ixs = append(ixs, ifxIxs...)
		if err := s.appendUnwrapWSOLIfNeeded(&ixs, in.OutputSettlement, bridgeOutMint, user, wsolMint); err != nil {
			return TxPlan{}, err
		}
		return TxPlan{
			User:               user,
			Tier:               tier,
			Instructions:       ixs,
			ServiceFeeLamports: serviceFee,
			SponsoredEligible:  sponsoredWired,
			ATACreates:         ata.count(),
		}, nil
	}, planned.Legs)
}

func (s *Service) loadPumpBuildState(
	baseMint solana.PublicKey,
	accounts map[solana.PublicKey]*rpc.Account,
) (bcPK, globalPK solana.PublicKey, curve pumpfun.BondingCurve, global pumpfun.Global, mintAcct *rpc.Account, quoteKind pumpfun.QuoteKind, err error) {
	bcPK, err = pumpfun.BondingCurvePDAFromMint(baseMint)
	if err != nil {
		return
	}
	globalPK, err = pumpfun.GlobalPDA(s.cfg)
	if err != nil {
		return
	}
	bcAcct := accounts[bcPK]
	globalAcct := accounts[globalPK]
	mintAcct = accounts[baseMint]
	if bcAcct == nil || globalAcct == nil || mintAcct == nil {
		err = fmt.Errorf("snapshot missing pump build accounts")
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

func (s *Service) resolveQuoteToken(
	baseMint solana.PublicKey,
	curve pumpfun.BondingCurve,
	accounts map[solana.PublicKey]*rpc.Account,
	wsolMint solana.PublicKey,
) (solana.PublicKey, solana.PublicKey, error) {
	quoteMint := pumpfun.EffectiveQuoteMint(s.cfg, curve.QuoteMint, wsolMint)
	mintAcct := accounts[quoteMint]
	if mintAcct == nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("snapshot missing quote mint %s", quoteMint)
	}
	return quoteMint, mintAcct.Owner, nil
}

func (s *Service) bridgeLegInstructions(
	ata *ataSetup,
	pool *bridge.DiscoveredPool,
	accounts map[solana.PublicKey]*rpc.Account,
	user solana.PublicKey,
	inputMint solana.PublicKey,
	amountIn, quotedOut uint64,
	slippageBPS int,
	skipInputATACreate bool,
) (swap solana.Instruction, err error) {
	return s.bridgeLegInstructionsWithMinOut(ata, pool, accounts, user, inputMint, amountIn, quotedOut, slippageBPS, 0, skipInputATACreate)
}

func minAmountOutForBridge(quotedOut uint64, slippageBPS int, override uint64) uint64 {
	if override > 0 {
		return override
	}
	return util.MinOut(quotedOut, slippageBPS)
}

func (s *Service) bridgeLegInstructionsWithMinOut(
	ata *ataSetup,
	pool *bridge.DiscoveredPool,
	accounts map[solana.PublicKey]*rpc.Account,
	user solana.PublicKey,
	inputMint solana.PublicKey,
	amountIn, quotedOut uint64,
	slippageBPS int,
	minAmountOut uint64,
	skipInputATACreate bool,
) (swap solana.Instruction, err error) {
	inPair, err := solpkg.DeriveATAPair(user, inputMint)
	if err != nil {
		return nil, err
	}
	outMint, err := solpkg.ParsePubkey(pool.OutputMint)
	if err != nil {
		return nil, err
	}
	outPair, err := solpkg.DeriveATAPair(user, outMint)
	if err != nil {
		return nil, err
	}
	inMintAcct := accounts[inputMint]
	outMintAcct := accounts[outMint]
	if inMintAcct == nil || outMintAcct == nil {
		return nil, fmt.Errorf("snapshot missing bridge mint accounts")
	}
	userInATA := solpkg.SelectATA(inPair, inMintAcct.Owner)
	userOutATA := solpkg.SelectATA(outPair, outMintAcct.Owner)

	if !skipInputATACreate {
		if err := ata.ensure(user, user, inputMint, inMintAcct.Owner); err != nil {
			return nil, err
		}
	}
	if err := ata.ensure(user, user, outMint, outMintAcct.Owner); err != nil {
		return nil, err
	}

	poolPK, err := solpkg.ParsePubkey(pool.PoolID)
	if err != nil {
		return nil, err
	}
	poolAcct := accounts[poolPK]
	if poolAcct == nil {
		return nil, fmt.Errorf("snapshot missing bridge pool %s", pool.PoolID)
	}
	router := bridge.NewRouter(s.cfg)
	swap, err = router.BuildSwap(bridge.SwapBuildParams{
		Pool:         pool,
		PoolAccount:  poolAcct,
		User:         user,
		InputATA:     userInATA,
		OutputATA:    userOutATA,
		AmountIn:     amountIn,
		MinAmountOut: minAmountOutForBridge(quotedOut, slippageBPS, minAmountOut),
	})
	if err != nil {
		return nil, err
	}
	return swap, nil
}

func (s *Service) appendPayAssetFeeTransfer(
	ixs *[]solana.Instruction,
	payMint solana.PublicKey,
	user, feePubkey solana.PublicKey,
	accounts map[solana.PublicKey]*rpc.Account,
	fee uint64,
) error {
	// Operator fee ATAs for supported pay mints are provisioned off-chain; never create them here.
	if fee == 0 {
		return nil
	}
	wsol, _ := solpkg.ParsePubkey(s.cfg.Quotes.WSOLMint)
	if payMint.Equals(wsol) {
		*ixs = append(*ixs, system.NewTransferInstruction(fee, user, feePubkey).Build())
		return nil
	}
	mintAcct := accounts[payMint]
	if mintAcct == nil {
		return fmt.Errorf("snapshot missing pay mint %s", payMint)
	}
	pair, err := solpkg.DeriveATAPair(user, payMint)
	if err != nil {
		return err
	}
	userATA := solpkg.SelectATA(pair, mintAcct.Owner)
	opATA, err := operatorQuoteATA(feePubkey, payMint, mintAcct.Owner)
	if err != nil {
		return err
	}
	*ixs = append(*ixs, token.NewTransferCheckedInstruction(
		fee,
		quoteDecimals(s.cfg, payMint.String()),
		userATA,
		payMint,
		opATA,
		user,
		[]solana.PublicKey{},
	).Build())
	return nil
}

// shouldWrapSOLForBridge is true when the bridge spends WSOL but the user pays native SOL.
func shouldWrapSOLForBridge(in QuoteInput, bridgeInMint, wsolMint solana.PublicKey, lamports uint64) bool {
	if lamports == 0 {
		return false
	}
	if in.InputSettlement == SettlementWSOLSPL {
		return false
	}
	return bridgeInMint.Equals(wsolMint)
}

func operatorQuoteATA(owner, mint, tokenProgram solana.PublicKey) (solana.PublicKey, error) {
	pair, err := solpkg.DeriveATAPair(owner, mint)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return solpkg.SelectATA(pair, tokenProgram), nil
}

func userQuoteATA(user, mint solana.PublicKey, accounts map[solana.PublicKey]*rpc.Account) (solana.PublicKey, error) {
	mintAcct := accounts[mint]
	if mintAcct == nil {
		return solana.PublicKey{}, fmt.Errorf("snapshot missing mint %s", mint)
	}
	pair, err := solpkg.DeriveATAPair(user, mint)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return solpkg.SelectATA(pair, mintAcct.Owner), nil
}

func computebudgetIx(tier config.PriorityFeeTier) []solana.Instruction {
	return []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(tier.ComputeUnitLimit).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(tier.MicroLamports).Build(),
	}
}

// prependComputeBudget strips any embedded compute-budget ixs and prepends one pair.
func prependComputeBudget(tier config.PriorityFeeTier, ixs []solana.Instruction) []solana.Instruction {
	out := computebudgetIx(tier)
	return append(out, stripComputeBudgetIxs(ixs)...)
}

func stripComputeBudgetIxs(ixs []solana.Instruction) []solana.Instruction {
	if len(ixs) == 0 {
		return ixs
	}
	out := make([]solana.Instruction, 0, len(ixs))
	for _, ix := range ixs {
		if ix.ProgramID().Equals(computebudget.ProgramID) {
			continue
		}
		out = append(out, ix)
	}
	return out
}
