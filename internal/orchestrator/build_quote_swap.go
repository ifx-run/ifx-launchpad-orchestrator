package orchestrator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/bridge"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	ifxpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/ifx"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/route"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/util"
)

func (s *Service) buildQuoteSwap(
	ctx context.Context,
	in QuoteInput,
	pool *bridge.DiscoveredPool,
	accounts map[solana.PublicKey]*rpc.Account,
) (*VariantsResult, error) {
	user, err := solpkg.ParsePubkey(in.UserPubkey)
	if err != nil {
		return nil, err
	}
	tier := s.cfg.Tier(in.PriorityTier)

	inMint, err := solpkg.ParsePubkey(in.InputMint)
	if err != nil {
		return nil, err
	}
	outMint, err := solpkg.ParsePubkey(in.OutputMint)
	if err != nil {
		return nil, err
	}
	inDec := quoteDecimals(s.cfg, in.InputMint)
	inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, inDec)
	if err != nil {
		return nil, err
	}
	outRaw, err := strconv.ParseUint(pool.OutAmount, 10, 64)
	if err != nil {
		return nil, err
	}
	minOut := util.MinOut(outRaw, in.SlippageBPS)

	poolPK, err := solpkg.ParsePubkey(pool.PoolID)
	if err != nil {
		return nil, err
	}
	poolAcct := accounts[poolPK]
	if poolAcct == nil {
		return nil, fmt.Errorf("snapshot missing bridge pool %s", pool.PoolID)
	}

	inPair, err := solpkg.DeriveATAPair(user, inMint)
	if err != nil {
		return nil, err
	}
	outPair, err := solpkg.DeriveATAPair(user, outMint)
	if err != nil {
		return nil, err
	}
	inMintAcct := accounts[inMint]
	if inMintAcct == nil {
		return nil, fmt.Errorf("snapshot missing input mint %s", in.InputMint)
	}
	outMintAcct := accounts[outMint]
	if outMintAcct == nil {
		return nil, fmt.Errorf("snapshot missing output mint %s", in.OutputMint)
	}
	userInATA := solpkg.SelectATA(inPair, inMintAcct.Owner)
	userOutATA := solpkg.SelectATA(outPair, outMintAcct.Owner)

	ata := newATASetup()
	if err := ata.ensure(user, user, inMint, inMintAcct.Owner); err != nil {
		return nil, err
	}
	if err := ata.ensure(user, user, outMint, outMintAcct.Owner); err != nil {
		return nil, err
	}
	wsolMint, err := solpkg.ParsePubkey(s.cfg.Quotes.WSOLMint)
	if err != nil {
		return nil, err
	}
	wrapBridgeSOL := shouldWrapSOLForBridge(in, inMint, wsolMint, inputRaw)
	if wrapBridgeSOL {
		if err := ata.ensure(user, user, wsolMint, solana.TokenProgramID); err != nil {
			return nil, err
		}
	}

	var wrapPreflight []solana.Instruction
	if wrapBridgeSOL {
		if err := appendWrapSOLDeposit(&wrapPreflight, user, user, wsolMint, solana.TokenProgramID, inputRaw); err != nil {
			return nil, err
		}
	}

	router := bridge.NewRouter(s.cfg)
	swapIx, err := router.BuildSwap(bridge.SwapBuildParams{
		Pool:              pool,
		PoolAccount:       poolAcct,
		User:              user,
		InputATA:          userInATA,
		OutputATA:         userOutATA,
		AmountIn:          inputRaw,
		MinAmountOut:      minOut,
		MintTokenPrograms: bridgeMintTokenPrograms(accounts, inMint, outMint),
	})
	if err != nil {
		return nil, err
	}

	var unwrap *solana.Instruction
	repayPartial := false
	if outMint.Equals(wsolMint) && wantsNativeSOL(in.OutputSettlement) {
		unwrapIx, err := solpkg.CloseWSOLATA(user, wsolMint, solana.TokenProgramID)
		if err != nil {
			return nil, err
		}
		unwrap = &unwrapIx
	} else if outMint.Equals(wsolMint) && in.OutputSettlement == SettlementWSOLSPL {
		repayPartial = true
	}

	legs := []route.Leg{{Kind: route.LegQuoteBridge, InputMint: in.InputMint, OutputMint: in.OutputMint}}
	sponsoredWired := route.QuoteSwapSponsoredEligible(
		in.InputMint, in.OutputMint, in.InputSettlement, in.OutputSettlement, s.cfg.Quotes.WSOLMint,
	)

	if !sponsoredWired {
		var ixs []solana.Instruction
		if err := ata.appendTo(&ixs, user); err != nil {
			return nil, err
		}
		ixs = append(ixs, wrapPreflight...)
		ixs = append(ixs, swapIx)
		if unwrap != nil {
			ixs = append(ixs, *unwrap)
		}
		return s.compileVariantsFromIXs(ctx, in, user, tier, ixs, 0, legs, false, ata.count())
	}

	bridgeParams := ifxpkg.QuoteBridgeParams{
		BridgeSwap:   swapIx,
		UnwrapWSOL:   unwrap,
		WSOLATA:      userOutATA,
		User:         user,
		TokenProgram: outMintAcct.Owner,
		RepayPartial: repayPartial,
	}

	return s.compilePreflightVariants(ctx, in, user, tier, 0, legs, ata.count(),
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
			preflight = append(preflight, wrapPreflight...)
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
				return ifxpkg.PlanQuoteBridgeSponsored(s.cfg, bridgeParams, repay, fixed, sponsor, ata.ifxSpecs())
			}
			var ixs []solana.Instruction
			ixs = append(ixs, swapIx)
			if unwrap != nil {
				ixs = append(ixs, *unwrap)
			}
			return ixs, nil
		}, true)
}

func quoteDecimals(cfg *config.Config, mint string) uint8 {
	if mint == cfg.Quotes.WSOLMint {
		return 9
	}
	return 6
}
