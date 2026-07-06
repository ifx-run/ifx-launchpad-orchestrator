package orchestrator

import (
	"context"

	ifxpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/ifx"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	"github.com/gagliardetto/solana-go"
)

type ifxVariantBuilder func(mode VariantMode) ([]solana.Instruction, error)
type preflightBuilder func(mode VariantMode) ([]solana.Instruction, error)

func (s *Service) compilePreflightVariants(
	ctx context.Context,
	in QuoteInput,
	user solana.PublicKey,
	tier config.PriorityFeeTier,
	serviceFee uint64,
	legs []route.Leg,
	ataCreates int,
	mkPreflight preflightBuilder,
	mkIfx ifxVariantBuilder,
	sponsoredWired bool,
) (*VariantsResult, error) {
	return s.compileAllVariants(ctx, in, func(mode VariantMode) (TxPlan, error) {
		preflight, err := mkPreflight(mode)
		if err != nil {
			return TxPlan{}, err
		}
		ifxIxs, err := mkIfx(mode)
		if err != nil {
			return TxPlan{}, err
		}
		ixs := make([]solana.Instruction, 0, len(preflight)+len(ifxIxs))
		ixs = append(ixs, preflight...)
		ixs = append(ixs, ifxIxs...)
		return TxPlan{
			User:               user,
			Tier:               tier,
			Instructions:       ixs,
			ServiceFeeLamports: serviceFee,
			SponsoredEligible:  sponsoredWired,
			ATACreates:         ataCreates,
		}, nil
	}, legs)
}

func (s *Service) sponsoredRepayParams(user solana.PublicKey) (ifxpkg.SponsoredRepayParams, error) {
	repayTo, err := s.sponsorRepayPubkey()
	if err != nil {
		return ifxpkg.SponsoredRepayParams{}, err
	}
	return ifxpkg.SponsoredRepayParams{User: user, RepayTo: repayTo}, nil
}

func (s *Service) fixedSponsoredRepay(tier config.PriorityFeeTier, mode VariantMode, ataCreates int) uint64 {
	jitoTip := jitoTipLamports(s.cfg, mode.Mev)
	numSigs := 1
	if mode.Sponsored {
		numSigs = 2
	}
	return EstimateRepayLamports(s.cfg, tier, numSigs, ataCreates, jitoTip)
}

// fixedSponsoredRepayFeesOnly is basic + priority + tip for on-chain repay when rent is measured via Ifx.
func (s *Service) fixedSponsoredRepayFeesOnly(tier config.PriorityFeeTier, mode VariantMode) uint64 {
	return s.fixedSponsoredRepay(tier, mode, 0)
}
