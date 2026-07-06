package orchestrator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/logx"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/snapshot"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/util"
)

func (s *Service) quoteSOLSettlement(ctx context.Context, in QuoteInput, start time.Time) (*QuoteResult, error) {
	wsolMint := s.cfg.Quotes.WSOLMint
	unwrap := route.SOLSettlementUnwrap(in.InputSettlement, in.OutputSettlement)
	legs := solSettlementLegs(wsolMint)

	inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, 9)
	if err != nil {
		return nil, err
	}
	if inputRaw == 0 {
		return nil, fmt.Errorf("input amount must be positive")
	}

	plan, err := snapshot.BuildFetchPlan(s.cfg, snapshot.FetchPlanInput{
		PairClass:  route.PairSOLSettlement,
		InputMint:  in.InputMint,
		OutputMint: in.OutputMint,
		UserPubkey: in.UserPubkey,
		Recipient:  recipientOrUser(in),
	})
	if err != nil {
		return nil, err
	}

	chainSnap, err := s.fetcher.Fetch(ctx, plan)
	if err != nil {
		return nil, err
	}

	if unwrap {
		user, err := solpkg.ParsePubkey(in.UserPubkey)
		if err != nil {
			return nil, err
		}
		wsolPK, err := solpkg.ParsePubkey(wsolMint)
		if err != nil {
			return nil, err
		}
		wsolATA, err := userWSOLATA(user, wsolPK, chainSnap.Accounts)
		if err != nil {
			return nil, err
		}
		bal, err := wsolATABalance(wsolATA, chainSnap.Accounts)
		if err != nil {
			return nil, err
		}
		if bal < inputRaw {
			return nil, fmt.Errorf("insufficient WSOL balance (%d < %d)", bal, inputRaw)
		}
	}

	output, minOut, gross, err := solSettlementQuoteAmounts(s, in, legs, inputRaw, in.SlippageBPS)
	if err != nil {
		return nil, err
	}

	result := &QuoteResult{
		Source:    "sol_settlement",
		PairClass: route.PairSOLSettlement.String(),
		Route:     legs,
		Snapshot: SnapshotMeta{
			AccountCount: len(plan.Keys),
			Commitment:   s.cfg.Snapshot.Commitment,
		},
		Quote: QuoteSummary{
			InputMint:         in.InputMint,
			OutputMint:        in.OutputMint,
			InputAmount:       in.InputAmount,
			OutputAmount:      output,
			MinOutputAmount:   minOut,
			GrossOutputAmount: gross,
		},
	}

	variants, err := s.buildSOLSettlement(ctx, in, chainSnap.Accounts, inputRaw)
	if err != nil {
		result.BuildSkippedReason = "build_error"
		result.BuildError = err.Error()
	} else {
		applyVariantsToQuote(result, variants)
	}

	logx.Info("quote", "sol settlement done",
		"unwrap", unwrap,
		"inputRaw", strconv.FormatUint(inputRaw, 10),
		"hasBuild", result.Build != nil,
		"ms", logx.Since(start),
	)
	return result, nil
}

func recipientOrUser(in QuoteInput) string {
	if in.RecipientPubkey != "" {
		return in.RecipientPubkey
	}
	return in.UserPubkey
}
