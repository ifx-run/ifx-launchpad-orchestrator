package orchestrator

import (
	"context"
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	ifxpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/ifx"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/util"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
)

type BuildResult struct {
	Transaction           string               `json:"transaction"`
	RecentBlockhash       string               `json:"recentBlockhash"`
	FeePayer              string               `json:"feePayer"`
	TransactionSizeBytes  int                  `json:"transactionSizeBytes"`
	ServiceFeeLamports    uint64               `json:"serviceFeeLamports,omitempty"`
	JitoTipLamports       uint64               `json:"jitoTipLamports,omitempty"`
	RepayEstimateLamports uint64               `json:"repayEstimateLamports,omitempty"`
	Variant               string               `json:"variant"`
	SettlementMode        string               `json:"settlementMode,omitempty"`
	Format                string               `json:"format"`
	Inspection            *solpkg.TxInspection `json:"inspection,omitempty"`
}

func (s *Service) BuildFromQuote(ctx context.Context, in QuoteInput, pairClass route.PairClass, accounts map[solana.PublicKey]*rpc.Account, detection *venue.Detection, baseMint solana.PublicKey) (*VariantsResult, error) {
	if in.UserPubkey == "" {
		return nil, fmt.Errorf("userPubkey required for build")
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

	switch detection.Venue {
	case venue.IDPumpfun:
		return s.buildPump(ctx, in, pairClass, accounts, baseMint, user, feePubkey, tier)
	default:
		return nil, fmt.Errorf("build not implemented for venue %s", detection.Venue)
	}
}

func (s *Service) buildPump(ctx context.Context, in QuoteInput, pairClass route.PairClass, accounts map[solana.PublicKey]*rpc.Account, baseMint, user, feePubkey solana.PublicKey, tier config.PriorityFeeTier) (*VariantsResult, error) {
	if pairClass == route.PairBuyLaunchpad && in.InputSettlement == SettlementWSOLSPL {
		return nil, fmt.Errorf("pump buy uses native SOL (lamports); WSOL SPL ATA is not supported in v1 — use native SOL or unwrap WSOL first")
	}
	bcPK, err := pumpfun.BondingCurvePDAFromMint(baseMint)
	if err != nil {
		return nil, err
	}
	globalPK, err := pumpfun.GlobalPDA(s.cfg)
	if err != nil {
		return nil, err
	}
	bcAcct := accounts[bcPK]
	globalAcct := accounts[globalPK]
	mintAcct := accounts[baseMint]
	if bcAcct == nil || globalAcct == nil || mintAcct == nil {
		return nil, fmt.Errorf("snapshot missing pump build accounts")
	}

	curve, err := pumpfun.DecodeBondingCurve(bcAcct.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	global, err := pumpfun.DecodeGlobal(globalAcct.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	wsolMint, err := solpkg.ParsePubkey(s.cfg.Quotes.WSOLMint)
	if err != nil {
		return nil, err
	}
	quoteKind := pumpfun.QuoteKindFor(s.cfg, curve)
	quoteMint, quoteTP, err := s.resolveQuoteToken(baseMint, curve, accounts, wsolMint)
	if err != nil {
		return nil, err
	}
	opFeeATA, err := operatorQuoteATA(feePubkey, quoteMint, quoteTP)
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
		PlatformFeePubkey:   feePubkey,
		PlatformFeeQuoteATA: opFeeATA,
		ComputeUnitLimit:    tier.ComputeUnitLimit,
		ComputeUnitPrice:    tier.MicroLamports,
	}

	baseDecimals, err := pumpfun.MintDecimals(mintAcct.Data.GetBinary())
	if err != nil {
		return nil, err
	}

	var serviceFeeLamports uint64
	var sellSOL bool
	switch pairClass {
	case route.PairBuyLaunchpad:
		quoteDec := quoteDecimals(s.cfg, quoteMint.String())
		inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, quoteDec)
		if err != nil {
			return nil, err
		}
		serviceFee := util.ApplyBPS(inputRaw, s.cfg.ServiceFee.BPS)
		if serviceFee >= inputRaw {
			return nil, fmt.Errorf("input too small after service fee")
		}
		netQuote := inputRaw - serviceFee
		legs := legsForPump(pairClass, in, quoteMint, baseMint)
		amounts, err := s.pumpBuyAmountsAfterRepay(in, legs, global, curve, netQuote, in.SlippageBPS, 1)
		if err != nil {
			return nil, err
		}
		params.SpendableQuoteIn = netQuote
		params.MinBaseOut = amounts.MinBaseOutGross
		switch quoteKind {
		case pumpfun.QuoteNativeSOL:
			params.ServiceFeeLamports = serviceFee
			serviceFeeLamports = serviceFee
		default:
			params.ServiceFeeQuote = serviceFee
		}
		return s.compilePumpBuyVariants(ctx, in, params, quoteKind, user, tier, serviceFeeLamports, legs, amounts.MinBaseOutSponsor)
	case route.PairSellLaunchpad:
		inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, baseDecimals)
		if err != nil {
			return nil, err
		}
		gross := pumpfun.SellQuoteOut(global, curve, inputRaw)
		minGross := util.MinOut(gross, in.SlippageBPS)
		params.BaseAmountIn = inputRaw
		params.MinQuoteOut = minGross
		serviceFee := util.ApplyBPS(minGross, s.cfg.ServiceFee.BPS)
		serviceFeeLamports = serviceFee
		switch quoteKind {
		case pumpfun.QuoteNativeSOL:
			sellSOL = ifxpkg.NeedsIfxForPumpSell(s.cfg.ServiceFee.BPS)
		default:
			params.ServiceFeeQuote = serviceFee
		}
		if sellSOL {
			return s.compilePumpSellSOLVariants(ctx, in, params, user, tier, serviceFeeLamports, legsForPump(pairClass, in, quoteMint, baseMint))
		}
		instructions, err := pumpfun.BuildSell(params, quoteKind)
		if err != nil {
			return nil, err
		}
		return s.compileSimpleVariants(ctx, in, user, tier, instructions, serviceFeeLamports, legsForPump(pairClass, in, quoteMint, baseMint), false)
	default:
		return nil, fmt.Errorf("unsupported pair class for pump build")
	}
}

func legsForPump(pairClass route.PairClass, in QuoteInput, quoteMint, baseMint solana.PublicKey) []route.Leg {
	switch pairClass {
	case route.PairSellLaunchpad:
		return []route.Leg{{Kind: route.LegLaunchpad, InputMint: baseMint.String(), OutputMint: quoteMint.String()}}
	case route.PairBuyLaunchpad:
		return []route.Leg{{Kind: route.LegLaunchpad, InputMint: in.InputMint, OutputMint: baseMint.String()}}
	default:
		return nil
	}
}

func (s *Service) compilePumpBuyVariants(ctx context.Context, in QuoteInput, params pumpfun.BuildParams, quoteKind pumpfun.QuoteKind, user solana.PublicKey, tier config.PriorityFeeTier, serviceFee uint64, legs []route.Leg, minBaseOutSponsor uint64) (*VariantsResult, error) {
	if quoteKind != pumpfun.QuoteNativeSOL {
		instructions, err := pumpfun.BuildBuy(params, quoteKind)
		if err != nil {
			return nil, err
		}
		return s.compileSimpleVariants(ctx, in, user, tier, instructions, serviceFee, legs, false)
	}

	var serviceFeeTransfer []solana.Instruction
	if serviceFee > 0 {
		serviceFeeTransfer = []solana.Instruction{
			system.NewTransferInstruction(serviceFee, user, params.PlatformFeePubkey).Build(),
		}
	}
	spendTplSponsored := params
	spendTplSponsored.SpendableQuoteIn = 0
	spendTplSponsored.MinBaseOut = minBaseOutSponsor
	buyTplSponsored, err := pumpfun.BuildBuyCoreIx(spendTplSponsored, quoteKind)
	if err != nil {
		return nil, err
	}

	return s.compilePreflightVariants(ctx, in, user, tier, serviceFee, legs, 1,
		func(mode VariantMode) ([]solana.Instruction, error) {
			if mode.Sponsored {
				return append([]solana.Instruction(nil), serviceFeeTransfer...), nil
			}
			payer, err := s.ataPayerForMode(mode, user)
			if err != nil {
				return nil, err
			}
			ataSetup, err := pumpfun.BuildBuySetupInstructionsWithPayer(params, quoteKind, payer)
			if err != nil {
				return nil, err
			}
			preflight := make([]solana.Instruction, 0, len(ataSetup)+len(serviceFeeTransfer))
			preflight = append(preflight, ataSetup...)
			preflight = append(preflight, serviceFeeTransfer...)
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
				return ifxpkg.PlanPumpBuySponsored(s.cfg, ifxpkg.PumpBuySponsoredParams{
					BuyTemplate:      buyTplSponsored,
					SpendableQuoteIn: params.SpendableQuoteIn,
					User:             user,
					SponsorPayer:     sponsor,
					ATACreates: []ifxpkg.ATASetupSpec{{
						Owner:        user,
						Mint:         params.BaseMint,
						TokenProgram: params.BaseTokenProgram,
					}},
					Repay:          repay,
					FixedRepayCost: fixed,
				})
			}
			buyIx, err := pumpfun.BuildBuyCoreIx(params, quoteKind)
			if err != nil {
				return nil, err
			}
			return []solana.Instruction{buyIx}, nil
		}, true)
}

func (s *Service) compilePumpSellSOLVariants(ctx context.Context, in QuoteInput, params pumpfun.BuildParams, user solana.PublicKey, tier config.PriorityFeeTier, serviceFee uint64, legs []route.Leg) (*VariantsResult, error) {
	return s.compileAllVariants(ctx, in, func(mode VariantMode) (TxPlan, error) {
		jitoTip := jitoTipLamports(s.cfg, mode.Mev)
		numSigs := 1
		if mode.Sponsored {
			numSigs = 2
		}
		fixedRepay := EstimateRepayLamports(s.cfg, tier, numSigs, 0, jitoTip)

		var core []solana.Instruction
		var err error
		if mode.Sponsored {
			repayTo, err := s.sponsorRepayPubkey()
			if err != nil {
				return TxPlan{}, err
			}
			core, err = ifxpkg.PlanPumpSellSponsored(s.cfg, params, s.cfg.ServiceFee.BPS, ifxpkg.SponsoredRepayParams{
				User:    user,
				RepayTo: repayTo,
			}, fixedRepay)
		} else {
			core, err = ifxpkg.PlanPumpSellWithSOLFee(s.cfg, params, s.cfg.ServiceFee.BPS)
		}
		if err != nil {
			return TxPlan{}, err
		}
		return TxPlan{
			User:               user,
			Tier:               tier,
			Instructions:       core,
			ServiceFeeLamports: serviceFee,
			SponsoredEligible:  true,
		}, nil
	}, legs)
}

func (s *Service) compileSimpleVariants(ctx context.Context, in QuoteInput, user solana.PublicKey, tier config.PriorityFeeTier, instructions []solana.Instruction, serviceFee uint64, legs []route.Leg, sponsoredEligible bool) (*VariantsResult, error) {
	return s.compileVariantsFromIXs(ctx, in, user, tier, instructions, serviceFee, legs, sponsoredEligible, 0)
}

func (s *Service) compileVariantsFromIXs(ctx context.Context, in QuoteInput, user solana.PublicKey, tier config.PriorityFeeTier, ixs []solana.Instruction, serviceFee uint64, legs []route.Leg, sponsoredEligible bool, ataCreates int) (*VariantsResult, error) {
	return s.compileAllVariants(ctx, in, func(mode VariantMode) (TxPlan, error) {
		return TxPlan{
			User:               user,
			Tier:               tier,
			Instructions:       ixs,
			ServiceFeeLamports: serviceFee,
			SponsoredEligible:  sponsoredEligible,
			ATACreates:         ataCreates,
		}, nil
	}, legs)
}
