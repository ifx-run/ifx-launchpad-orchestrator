package orchestrator

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/util"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
)

func (s *Service) buildSOLSettlement(
	ctx context.Context,
	in QuoteInput,
	accounts map[solana.PublicKey]*rpc.Account,
	inputRaw uint64,
) (*VariantsResult, error) {
	user, err := solpkg.ParsePubkey(in.UserPubkey)
	if err != nil {
		return nil, err
	}
	tier := settlementNoPriorityTier()
	wsolMint, err := solpkg.ParsePubkey(s.cfg.Quotes.WSOLMint)
	if err != nil {
		return nil, err
	}
	unwrap := route.SOLSettlementUnwrap(in.InputSettlement, in.OutputSettlement)
	legs := solSettlementLegs(in.InputMint)

	if unwrap {
		return s.buildWSOLUnwrap(ctx, in, user, tier, wsolMint, accounts, inputRaw, legs)
	}
	return s.buildSOLWrap(ctx, in, user, tier, wsolMint, inputRaw, legs)
}

func solSettlementLegs(wsolMint string) []route.Leg {
	return []route.Leg{{
		Kind:       route.LegSOLSettlement,
		InputMint:  wsolMint,
		OutputMint: wsolMint,
	}}
}

func wsolUnwrapModeFor(settleMode string) solpkg.WSOLUnwrapMode {
	switch settleMode {
	case SettlementModeClose:
		return solpkg.WSOLUnwrapClose
	case SettlementModeUnwrapAll:
		return solpkg.WSOLUnwrapLamportsAll
	default:
		return solpkg.WSOLUnwrapPartial
	}
}

func (s *Service) buildSOLWrap(
	ctx context.Context,
	in QuoteInput,
	user solana.PublicKey,
	tier config.PriorityFeeTier,
	wsolMint solana.PublicKey,
	inputRaw uint64,
	legs []route.Leg,
) (*VariantsResult, error) {
	wrapIxs, err := solpkg.WrapSOLInstructions(user, user, wsolMint, solana.TokenProgramID, inputRaw)
	if err != nil {
		return nil, err
	}
	ataCreates := 1
	return s.compileSettlementBuilds(ctx, in, legs, nil, false, func(mode VariantMode, _ string) (TxPlan, error) {
		return TxPlan{
			User:              user,
			Tier:              tier,
			Instructions:      wrapIxs,
			SponsoredEligible: false,
			ATACreates:        ataCreates,
		}, nil
	}, settlementVariantPolicy{unwrap: false})
}

func (s *Service) buildWSOLUnwrap(
	ctx context.Context,
	in QuoteInput,
	user solana.PublicKey,
	tier config.PriorityFeeTier,
	wsolMint solana.PublicKey,
	accounts map[solana.PublicKey]*rpc.Account,
	inputRaw uint64,
	legs []route.Leg,
) (*VariantsResult, error) {
	wsolATA, err := userWSOLATA(user, wsolMint, accounts)
	if err != nil {
		return nil, err
	}
	bal, err := wsolATABalance(wsolATA, accounts)
	if err != nil {
		return nil, err
	}
	if bal < inputRaw {
		return nil, fmt.Errorf("insufficient WSOL balance (%d < %d)", bal, inputRaw)
	}

	fullBalance := inputRaw == bal
	var settlementModes []string
	if fullBalance {
		settlementModes = []string{SettlementModeClose, SettlementModeUnwrapAll}
	}

	return s.compileSettlementBuilds(ctx, in, legs, settlementModes, fullBalance, func(mode VariantMode, settleMode string) (TxPlan, error) {
		unwrapMode := wsolUnwrapModeFor(settleMode)
		if mode.Sponsored {
			repay := s.fixedSponsoredRepayFeesOnly(settlementNoPriorityTier(), mode)
			ixs, err := s.buildSponsoredUnwrapInstructions(user, wsolMint, inputRaw, unwrapMode, repay)
			if err != nil {
				return TxPlan{}, err
			}
			return TxPlan{
				User:              user,
				Tier:              tier,
				Instructions:      ixs,
				SponsoredEligible: true,
			}, nil
		}
		ixs, err := solpkg.UnwrapWSOLInstructions(user, wsolMint, solana.TokenProgramID, inputRaw, unwrapMode)
		if err != nil {
			return TxPlan{}, err
		}
		return TxPlan{
			User:              user,
			Tier:              tier,
			Instructions:      ixs,
			SponsoredEligible: false,
		}, nil
	}, settlementVariantPolicy{unwrap: true})
}

func userWSOLATA(user, wsolMint solana.PublicKey, accounts map[solana.PublicKey]*rpc.Account) (solana.PublicKey, error) {
	pair, err := solpkg.DeriveATAPair(user, wsolMint)
	if err != nil {
		return solana.PublicKey{}, err
	}
	mintAcct := accounts[wsolMint]
	if mintAcct == nil {
		return solana.PublicKey{}, fmt.Errorf("snapshot missing WSOL mint")
	}
	return solpkg.SelectATA(pair, mintAcct.Owner), nil
}

func wsolATABalance(ata solana.PublicKey, accounts map[solana.PublicKey]*rpc.Account) (uint64, error) {
	acct := accounts[ata]
	if acct == nil {
		return 0, fmt.Errorf("WSOL ATA not found; wrap SOL first or fund WSOL")
	}
	return solpkg.TokenAccountAmount(acct.Data.GetBinary())
}

// buildSponsoredUnwrapInstructions appends a fixed offline repay transfer after unwrap.
// No Ifx: sol_settlement has no priority fee, tip, or sponsor-paid rent.
func (s *Service) buildSponsoredUnwrapInstructions(
	user, wsolMint solana.PublicKey,
	lamports uint64,
	mode solpkg.WSOLUnwrapMode,
	repayLamports uint64,
) ([]solana.Instruction, error) {
	if repayLamports > 0 && lamports <= repayLamports {
		return nil, fmt.Errorf("insufficient unwrap amount (%d) for sponsor repay (%d)", lamports, repayLamports)
	}
	ixs, err := solpkg.UnwrapWSOLInstructions(user, wsolMint, solana.TokenProgramID, lamports, mode)
	if err != nil {
		return nil, err
	}
	if repayLamports == 0 {
		return ixs, nil
	}
	repayTo, err := s.sponsorRepayPubkey()
	if err != nil {
		return nil, err
	}
	return append(ixs, system.NewTransferInstruction(repayLamports, user, repayTo).Build()), nil
}

type settlementVariantPolicy struct {
	unwrap bool
}

type settlementPlanBuilder func(mode VariantMode, settleMode string) (TxPlan, error)

func (s *Service) compileSettlementBuilds(
	ctx context.Context,
	in QuoteInput,
	legs []route.Leg,
	settlementModes []string,
	fullBalance bool,
	mkPlan settlementPlanBuilder,
	policy settlementVariantPolicy,
) (*VariantsResult, error) {
	out := newVariantsResult()
	out.SettlementFullBalance = fullBalance
	out.SettlementModes = settlementModes
	if fullBalance {
		out.DefaultVariant = settlementBuildKey(VariantSelfFunded, SettlementModeClose)
	}

	bh, err := s.solana.LatestBlockhash(ctx)
	if err != nil {
		return nil, err
	}
	blockhash, err := solana.HashFromBase58(bh.Hash)
	if err != nil {
		return nil, err
	}

	var sponsor *SponsorSigner
	if policy.unwrap && s.sponsoredSwapEligible(in, legs) {
		sponsor, _ = loadSponsorSigner(s.cfg)
	}

	modeKeys := settlementModes
	if len(modeKeys) == 0 {
		modeKeys = []string{""}
	}

	for _, settleMode := range modeKeys {
		for _, mode := range AllVariantModes() {
			key := settlementBuildKey(mode.Key(), settleMode)

			if mode.Mev {
				out.Capabilities[key] = Capability{Supported: false, Reason: "sol_settlement_no_mev"}
				continue
			}
			if mode.Sponsored {
				if !policy.unwrap {
					out.Capabilities[key] = Capability{Supported: false, Reason: "wrap_user_pays"}
					continue
				}
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

			plan, err := mkPlan(mode, settleMode)
			if err != nil {
				out.Capabilities[key] = Capability{Supported: false, Reason: "build_error"}
				continue
			}
			if mode.Sponsored && !plan.SponsoredEligible {
				out.Capabilities[key] = Capability{Supported: false, Reason: "sponsored_not_wired"}
				continue
			}

			ixs := append([]solana.Instruction(nil), plan.Instructions...)
			feePayer := plan.User
			if mode.Sponsored {
				feePayer = sponsor.Pubkey
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
				repayEstimate = EstimateRepayLamports(s.cfg, settlementNoPriorityTier(), numSigs, plan.ATACreates, 0)
			}

			out.Builds[key] = &BuildResult{
				Transaction:           txB64,
				RecentBlockhash:       compiled.RecentBlockhash,
				FeePayer:              compiled.FeePayer,
				TransactionSizeBytes:  compiled.TransactionSize,
				ServiceFeeLamports:    plan.ServiceFeeLamports,
				Variant:               mode.Key(),
				SettlementMode:        settleMode,
				Format:                "v0",
				Inspection:            solpkg.InspectTransaction(tx, compiled.TransactionSize),
				RepayEstimateLamports: repayEstimate,
			}
			out.Capabilities[key] = Capability{Supported: true}
		}
	}

	if len(out.Builds) == 0 {
		return out, fmt.Errorf("no build variants fit within tx size limits")
	}
	return out, nil
}

func solSettlementQuoteAmounts(s *Service, in QuoteInput, legs []route.Leg, inputRaw uint64, slippageBPS int) (output, minOut, gross string, err error) {
	gross = fmt.Sprintf("%d", inputRaw)
	output = gross
	min := util.MinOut(inputRaw, slippageBPS)
	minOut = fmt.Sprintf("%d", min)

	if !route.SOLSettlementUnwrap(in.InputSettlement, in.OutputSettlement) {
		return output, minOut, gross, nil
	}
	if !s.shouldDeductSponsorRepayInQuote(in, legs) {
		return output, minOut, gross, nil
	}
	repay := s.quoteSettlementRepayDeduction(0)
	net, err := subtractRepayFromPumpIn(inputRaw, repay)
	if err != nil {
		return "", "", "", err
	}
	minNet := util.MinOut(net, slippageBPS)
	minOut = fmt.Sprintf("%d", minNet)
	return output, minOut, gross, nil
}
