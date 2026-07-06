package orchestrator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/bridge"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/jupiter"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/logx"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/snapshot"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/util"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func plannedLaunchpadRoute(in QuoteInput, pairClass route.PairClass, qNative string, isQuote func(string) bool) route.PlannedRoute {
	switch pairClass {
	case route.PairBuyLaunchpad:
		return route.PlanLaunchpadRoute(pairClass, in.InputMint, in.OutputMint, qNative, isQuote)
	case route.PairSellLaunchpad:
		return route.PlanLaunchpadRoute(pairClass, in.InputMint, in.OutputMint, qNative, isQuote)
	case route.PairSwapLaunchpad:
		return route.PlanLaunchpadRoute(pairClass, in.InputMint, in.OutputMint, qNative, isQuote)
	default:
		return route.PlannedRoute{}
	}
}

func legsFromPlanned(p route.PlannedRoute, venue string, bridgePoolID, bridgePoolType string) []route.Leg {
	out := make([]route.Leg, len(p.Legs))
	for i, leg := range p.Legs {
		out[i] = leg
		if leg.Kind == route.LegLaunchpad {
			out[i].Venue = venue
		}
		if leg.Kind == route.LegQuoteBridge && bridgePoolID != "" {
			out[i].PoolID = bridgePoolID
			out[i].PoolType = bridgePoolType
		}
	}
	return out
}

func bridgeLegFromPlanned(planned route.PlannedRoute) (route.Leg, error) {
	for _, leg := range planned.Legs {
		if leg.Kind == route.LegQuoteBridge {
			return leg, nil
		}
	}
	return route.Leg{}, fmt.Errorf("planned route missing bridge leg")
}

func (s *Service) ensureBridgePoolAccount(ctx context.Context, snap *snapshot.ChainSnapshot, poolID string) error {
	poolPK, err := solpkg.ParsePubkey(poolID)
	if err != nil {
		return err
	}
	if snap.HasAccount(poolPK) {
		return nil
	}
	logx.Debug("bridge", "fetch missing pool account", "poolId", poolID)
	commitment := solpkg.SnapshotCommitment(s.cfg.Snapshot.Commitment)
	accounts, err := s.solana.FetchAccounts(ctx, []solana.PublicKey{poolPK}, commitment, 1)
	if err != nil {
		return err
	}
	if accounts[poolPK] == nil {
		return fmt.Errorf("bridge pool account %s not found", poolID)
	}
	snap.Accounts[poolPK] = accounts[poolPK]
	return nil
}

func (s *Service) discoverBridgeForPlanned(
	ctx context.Context,
	in QuoteInput,
	pairClass route.PairClass,
	planned route.PlannedRoute,
	snap *snapshot.ChainSnapshot,
	baseMint solana.PublicKey,
) (*bridge.DiscoveredPool, error) {
	bridgeLeg, err := bridgeLegFromPlanned(planned)
	if err != nil {
		return nil, err
	}
	amount, err := s.bridgeDiscoverAmount(in, pairClass, snap, baseMint, bridgeLeg)
	if err != nil {
		return nil, err
	}
	logx.Info("bridge", "discover bridge leg",
		"pairClass", pairClass.String(),
		"inputMint", bridgeLeg.InputMint,
		"outputMint", bridgeLeg.OutputMint,
		"amount", amount,
		"hopCount", planned.HopCount,
	)
	pool, err := s.jupiter.DiscoverSingleHop(ctx, jupiter.DiscoverRequest{
		InputMint:   bridgeLeg.InputMint,
		OutputMint:  bridgeLeg.OutputMint,
		Amount:      strconv.FormatUint(amount, 10),
		SlippageBPS: in.SlippageBPS,
	})
	if err != nil {
		return nil, err
	}
	if err := s.ensureBridgePoolAccount(ctx, snap, pool.PoolID); err != nil {
		return nil, err
	}
	return pool, nil
}

func (s *Service) bridgeDiscoverAmount(
	in QuoteInput,
	pairClass route.PairClass,
	snap *snapshot.ChainSnapshot,
	baseMint solana.PublicKey,
	bridgeLeg route.Leg,
) (uint64, error) {
	switch pairClass {
	case route.PairBuyLaunchpad:
		payDec := quoteDecimals(s.cfg, in.InputMint)
		inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, payDec)
		if err != nil {
			return 0, err
		}
		fee := util.ApplyBPS(inputRaw, s.cfg.ServiceFee.BPS)
		if fee >= inputRaw {
			return 0, fmt.Errorf("input too small after service fee")
		}
		return inputRaw - fee, nil
	case route.PairSellLaunchpad:
		outcome, err := pumpfun.QuoteFromAccounts(s.cfg, snap.Accounts, baseMint, pumpfun.QuoteParams{
			PairClass:      pairClass,
			InputMint:      in.InputMint,
			OutputMint:     in.OutputMint,
			InputAmount:    in.InputAmount,
			InputAmountRaw: in.InputAmountRaw,
			SlippageBPS:    in.SlippageBPS,
			ServiceFeeBPS:  s.cfg.ServiceFee.BPS,
		})
		if err != nil {
			return 0, err
		}
		if outcome.GrossOutputAmount == 0 {
			return 0, fmt.Errorf("launchpad sell produced zero quote")
		}
		minGross := util.MinOut(outcome.GrossOutputAmount, in.SlippageBPS)
		fee := util.ApplyBPS(minGross, s.cfg.ServiceFee.BPS)
		if fee >= minGross {
			return 0, fmt.Errorf("output too small after service fee")
		}
		return minGross - fee, nil
	default:
		return 0, fmt.Errorf("bridge discover amount unsupported for %s", pairClass)
	}
}

type launchpadQuoteOutcome struct {
	OutputAmount      uint64
	MinOutputAmount   uint64
	GrossOutputAmount uint64
	ServiceFeeAmount  uint64
	ServiceFeeBasis   string
	PriceImpact       string
}

func (s *Service) quotePumpLaunchpad(
	in QuoteInput,
	pairClass route.PairClass,
	planned route.PlannedRoute,
	bridgePool *bridge.DiscoveredPool,
	snap *snapshot.ChainSnapshot,
	baseMint solana.PublicKey,
) (launchpadQuoteOutcome, error) {
	if planned.HopCount <= 1 {
		outcome, err := pumpfun.QuoteFromAccounts(s.cfg, snap.Accounts, baseMint, pumpfun.QuoteParams{
			PairClass:      pairClass,
			InputMint:      in.InputMint,
			OutputMint:     in.OutputMint,
			InputAmount:    in.InputAmount,
			InputAmountRaw: in.InputAmountRaw,
			SlippageBPS:    in.SlippageBPS,
			ServiceFeeBPS:  s.cfg.ServiceFee.BPS,
		})
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		basis := ""
		switch pairClass {
		case route.PairBuyLaunchpad:
			basis = "input_quote"
			if pairClass == route.PairBuyLaunchpad && s.shouldDeductSponsorRepayInQuote(in, planned.Legs) {
				bcPK, err := pumpfun.BondingCurvePDAFromMint(baseMint)
				if err != nil {
					return launchpadQuoteOutcome{}, err
				}
				globalPK, err := pumpfun.GlobalPDA(s.cfg)
				if err != nil {
					return launchpadQuoteOutcome{}, err
				}
				bcAcct := snap.Accounts[bcPK]
				globalAcct := snap.Accounts[globalPK]
				if bcAcct == nil || globalAcct == nil {
					return launchpadQuoteOutcome{}, fmt.Errorf("snapshot missing pump quote accounts")
				}
				curve, err := pumpfun.DecodeBondingCurve(bcAcct.Data.GetBinary())
				if err != nil {
					return launchpadQuoteOutcome{}, err
				}
				global, err := pumpfun.DecodeGlobal(globalAcct.Data.GetBinary())
				if err != nil {
					return launchpadQuoteOutcome{}, err
				}
				if pumpfun.QuoteKindFor(s.cfg, curve) == pumpfun.QuoteNativeSOL {
					quoteDec := quoteDecimals(s.cfg, in.InputMint)
					inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, quoteDec)
					if err != nil {
						return launchpadQuoteOutcome{}, err
					}
					fee := util.ApplyBPS(inputRaw, s.cfg.ServiceFee.BPS)
					if fee >= inputRaw {
						return launchpadQuoteOutcome{}, fmt.Errorf("input too small after service fee")
					}
					netQuote := inputRaw - fee
					amounts, err := s.pumpBuyAmountsAfterRepay(in, planned.Legs, global, curve, netQuote, in.SlippageBPS, 1)
					if err != nil {
						return launchpadQuoteOutcome{}, err
					}
					if amounts.RepayDeducted > 0 {
						out := pumpfun.BuyBaseOut(global, curve, amounts.SponsoredPumpIn)
						outcome.OutputAmount = out
						outcome.MinOutputAmount = amounts.MinBaseOutSponsor
					}
				}
			}
		case route.PairSellLaunchpad:
			basis = "min_gross_quote"
		}
		return launchpadQuoteOutcome{
			OutputAmount:      outcome.OutputAmount,
			MinOutputAmount:   outcome.MinOutputAmount,
			GrossOutputAmount: outcome.GrossOutputAmount,
			ServiceFeeAmount:  outcome.ServiceFeeAmount,
			ServiceFeeBasis:   basis,
		}, nil
	}
	if pairClass == route.PairSwapLaunchpad {
		mintB, err := solpkg.ParsePubkey(in.OutputMint)
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		return s.quotePumpSwap(in, snap, baseMint, mintB)
	}
	if bridgePool == nil {
		return launchpadQuoteOutcome{}, fmt.Errorf("bridge pool required for multi-leg quote")
	}

	switch pairClass {
	case route.PairBuyLaunchpad:
		payDec := quoteDecimals(s.cfg, in.InputMint)
		inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, payDec)
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		fee := util.ApplyBPS(inputRaw, s.cfg.ServiceFee.BPS)
		if fee >= inputRaw {
			return launchpadQuoteOutcome{}, fmt.Errorf("input too small after service fee")
		}
		bridgeOut, err := strconv.ParseUint(bridgePool.OutAmount, 10, 64)
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		bcPK, err := pumpfun.BondingCurvePDAFromMint(baseMint)
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		globalPK, err := pumpfun.GlobalPDA(s.cfg)
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		bcAcct := snap.Accounts[bcPK]
		globalAcct := snap.Accounts[globalPK]
		if bcAcct == nil || globalAcct == nil {
			return launchpadQuoteOutcome{}, fmt.Errorf("snapshot missing pump quote accounts")
		}
		curve, err := pumpfun.DecodeBondingCurve(bcAcct.Data.GetBinary())
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		global, err := pumpfun.DecodeGlobal(globalAcct.Data.GetBinary())
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		wrapBridge := planned.Legs[0].OutputMint == s.cfg.Quotes.WSOLMint &&
			in.InputSettlement != SettlementWSOLSPL &&
			in.InputMint != s.cfg.Quotes.WSOLMint
		amounts, err := s.pumpBuyAmountsAfterRepay(
			in, planned.Legs, global, curve, bridgeOut, in.SlippageBPS,
			estimatePumpBuyTwoLegATACreates(wrapBridge),
		)
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		pumpIn := amounts.GrossPumpIn
		minOut := amounts.MinBaseOutGross
		if amounts.RepayDeducted > 0 {
			pumpIn = amounts.SponsoredPumpIn
			minOut = amounts.MinBaseOutSponsor
		}
		out := pumpfun.BuyBaseOut(global, curve, pumpIn)
		return launchpadQuoteOutcome{
			OutputAmount:     out,
			MinOutputAmount:  minOut,
			ServiceFeeAmount: fee,
			ServiceFeeBasis:  "input_pay_mint",
			PriceImpact:      bridgePool.PriceImpact,
		}, nil
	case route.PairSellLaunchpad:
		launchOutcome, err := pumpfun.QuoteFromAccounts(s.cfg, snap.Accounts, baseMint, pumpfun.QuoteParams{
			PairClass:      pairClass,
			InputMint:      in.InputMint,
			OutputMint:     in.OutputMint,
			InputAmount:    in.InputAmount,
			InputAmountRaw: in.InputAmountRaw,
			SlippageBPS:    in.SlippageBPS,
			ServiceFeeBPS:  s.cfg.ServiceFee.BPS,
		})
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		bridgeOut, err := strconv.ParseUint(bridgePool.OutAmount, 10, 64)
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		minGross := util.MinOut(launchOutcome.GrossOutputAmount, in.SlippageBPS)
		fee := util.ApplyBPS(minGross, s.cfg.ServiceFee.BPS)
		grossBridgeIn := minGross - fee
		amounts, err := s.sellBridgeAmountsAfterRepay(
			in, planned.Legs, grossBridgeIn, bridgeOut, in.SlippageBPS,
			estimatePumpSellTwoLegATACreates(pumpfun.QuoteNativeSOL),
		)
		if err != nil {
			return launchpadQuoteOutcome{}, err
		}
		out := bridgeOut
		minOut := amounts.MinOutGross
		if amounts.RepayDeducted > 0 {
			out = scaleBridgeOut(bridgeOut, grossBridgeIn, amounts.SponsoredBridgeIn)
			minOut = amounts.MinOutSponsor
		}
		return launchpadQuoteOutcome{
			OutputAmount:      out,
			MinOutputAmount:   minOut,
			GrossOutputAmount: launchOutcome.GrossOutputAmount,
			ServiceFeeAmount:  launchOutcome.ServiceFeeAmount,
			ServiceFeeBasis:   "min_gross_quote",
			PriceImpact:       bridgePool.PriceImpact,
		}, nil
	default:
		return launchpadQuoteOutcome{}, fmt.Errorf("multi-leg quote unsupported for %s", pairClass)
	}
}

func (s *Service) quotePumpSwap(
	in QuoteInput,
	snap *snapshot.ChainSnapshot,
	mintA, mintB solana.PublicKey,
) (launchpadQuoteOutcome, error) {
	outcome, err := pumpfun.QuoteSwapAB(s.cfg, snap.Accounts, mintA, mintB, pumpfun.QuoteParams{
		PairClass:      route.PairSwapLaunchpad,
		InputMint:      in.InputMint,
		OutputMint:     in.OutputMint,
		InputAmount:    in.InputAmount,
		InputAmountRaw: in.InputAmountRaw,
		SlippageBPS:    in.SlippageBPS,
		ServiceFeeBPS:  s.cfg.ServiceFee.BPS,
	})
	if err != nil {
		return launchpadQuoteOutcome{}, err
	}
	return launchpadQuoteOutcome{
		OutputAmount:      outcome.OutputAmount,
		MinOutputAmount:   outcome.MinOutputAmount,
		GrossOutputAmount: outcome.GrossOutputAmount,
		ServiceFeeAmount:  outcome.ServiceFeeAmount,
		ServiceFeeBasis:   "min_gross_quote",
	}, nil
}

func applyLaunchpadQuote(summary *QuoteSummary, q launchpadQuoteOutcome) {
	summary.OutputAmount = strconv.FormatUint(q.OutputAmount, 10)
	summary.MinOutputAmount = strconv.FormatUint(q.MinOutputAmount, 10)
	if q.GrossOutputAmount > 0 {
		summary.GrossOutputAmount = strconv.FormatUint(q.GrossOutputAmount, 10)
	}
	if q.ServiceFeeAmount > 0 {
		summary.ServiceFeeRaw = strconv.FormatUint(q.ServiceFeeAmount, 10)
	}
	summary.ServiceFeeBasis = q.ServiceFeeBasis
	if q.PriceImpact != "" {
		summary.PriceImpact = q.PriceImpact
	}
}

func (s *Service) buildPumpFromPlanned(
	ctx context.Context,
	in QuoteInput,
	pairClass route.PairClass,
	planned route.PlannedRoute,
	bridgePool *bridge.DiscoveredPool,
	accounts map[solana.PublicKey]*rpc.Account,
	detection *venue.Detection,
	baseMint solana.PublicKey,
) (*VariantsResult, error) {
	if planned.HopCount > 1 {
		if pairClass == route.PairSwapLaunchpad {
			mintB, err := solpkg.ParsePubkey(in.OutputMint)
			if err != nil {
				return nil, err
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
			return s.buildPumpSwapTwoLeg(ctx, in, planned, accounts, baseMint, mintB, user, feePubkey, tier)
		}
		return s.buildPumpMultiLeg(ctx, in, pairClass, planned, bridgePool, accounts, detection, baseMint)
	}
	return s.BuildFromQuote(ctx, in, pairClass, accounts, detection, baseMint)
}
