package orchestrator

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/route"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
)

// PlanBuilder assembles instructions for a specific variant mode.
type PlanBuilder func(mode VariantMode) (TxPlan, error)

func (s *Service) sponsoredSwapEligible(in QuoteInput, legs []route.Leg) bool {
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

func (s *Service) compileAllVariants(ctx context.Context, in QuoteInput, mkPlan PlanBuilder, legs []route.Leg) (*VariantsResult, error) {
	out := newVariantsResult()

	bh, err := s.solana.LatestBlockhash(ctx)
	if err != nil {
		return nil, err
	}
	blockhash, err := solana.HashFromBase58(bh.Hash)
	if err != nil {
		return nil, err
	}

	var sponsor *SponsorSigner
	if s.sponsoredSwapEligible(in, legs) {
		sponsor, _ = loadSponsorSigner(s.cfg)
	}

	for _, mode := range AllVariantModes() {
		key := mode.Key()
		if mode.Mev && !s.cfg.Jito.Enabled {
			out.Capabilities[key] = Capability{Supported: false, Reason: "jito_disabled"}
			continue
		}
		if mode.Sponsored {
			if !s.sponsoredSwapEligible(in, legs) {
				out.Capabilities[key] = Capability{Supported: false, Reason: "no_sol_in_route"}
				continue
			}
			if !s.cfg.Sponsor.Enabled {
				out.Capabilities[key] = Capability{Supported: false, Reason: "sponsor_disabled"}
				continue
			}
			if sponsor == nil {
				out.Capabilities[key] = Capability{Supported: false, Reason: "sponsor_keypair"}
				continue
			}
		}

		plan, err := mkPlan(mode)
		if err != nil {
			out.Capabilities[key] = Capability{Supported: false, Reason: "build_error"}
			continue
		}
		if mode.Sponsored && !plan.SponsoredEligible {
			out.Capabilities[key] = Capability{Supported: false, Reason: "sponsored_not_wired"}
			continue
		}

		ixs := prependComputeBudget(plan.Tier, plan.Instructions)
		feePayer := plan.User
		jitoTip := jitoTipLamports(s.cfg, mode.Mev)

		if mode.Sponsored {
			feePayer = sponsor.Pubkey
		}
		if mode.Mev && s.cfg.Jito.Enabled {
			tipIx, err := JitoTipInstruction(s.cfg, feePayer, jitoTip)
			if err != nil {
				out.Capabilities[key] = Capability{Supported: false, Reason: "jito_tip"}
				continue
			}
			ixs = append(ixs, tipIx)
		}

		tx, compiled, err := solpkg.CompileV0Tx(feePayer, blockhash, ixs, s.solana.AddressLookupTables())
		if err != nil {
			out.Capabilities[key] = Capability{Supported: false, Reason: "compile_error"}
			continue
		}
		if compiled.TransactionSize > s.cfg.Tx.MaxBytes {
			out.Capabilities[key] = Capability{Supported: false, Reason: "tx_too_large"}
			continue
		}

		txB64 := compiled.Transaction
		if mode.Sponsored {
			if _, err := tx.PartialSign(func(key solana.PublicKey) *solana.PrivateKey {
				if key.Equals(sponsor.Pubkey) {
					return &sponsor.Key
				}
				return nil
			}); err != nil {
				out.Capabilities[key] = Capability{Supported: false, Reason: "sponsor_sign"}
				continue
			}
			raw, err := tx.MarshalBinary()
			if err != nil {
				out.Capabilities[key] = Capability{Supported: false, Reason: "marshal_error"}
				continue
			}
			txB64 = base64.StdEncoding.EncodeToString(raw)
		}

		numSigs := 1
		if mode.Sponsored {
			numSigs = 2
		}
		var repayEstimate uint64
		if mode.Sponsored {
			repayEstimate = EstimateRepayLamports(s.cfg, plan.Tier, numSigs, plan.ATACreates, jitoTip)
		}

		out.Builds[key] = &BuildResult{
			Transaction:           txB64,
			RecentBlockhash:       compiled.RecentBlockhash,
			FeePayer:              compiled.FeePayer,
			TransactionSizeBytes:  compiled.TransactionSize,
			ServiceFeeLamports:    plan.ServiceFeeLamports,
			Variant:               key,
			Format:                "v0",
			Inspection:            solpkg.InspectTransaction(tx, compiled.TransactionSize),
			JitoTipLamports:       jitoTip,
			RepayEstimateLamports: repayEstimate,
		}
		out.Capabilities[key] = Capability{Supported: true}
	}

	if len(out.Builds) == 0 {
		return out, fmt.Errorf("no build variants fit within tx size limits")
	}
	return out, nil
}

func applyVariantsToQuote(result *QuoteResult, variants *VariantsResult) {
	if variants == nil {
		return
	}
	result.Builds = variants.Builds
	result.Capabilities = variants.Capabilities
	result.SettlementFullBalance = variants.SettlementFullBalance
	result.SettlementModes = variants.SettlementModes
	if b := variants.Builds[variants.DefaultVariant]; b != nil {
		result.Build = b
	} else {
		for _, b := range variants.Builds {
			result.Build = b
			break
		}
	}
}
