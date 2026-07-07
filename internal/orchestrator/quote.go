package orchestrator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/bridge"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/jupiter"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/logx"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/route"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/snapshot"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/util"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

type Service struct {
	cfg     *config.Config
	solana  *solpkg.Client
	jupiter *jupiter.Client
	fetcher *snapshot.Fetcher
}

func NewService(cfg *config.Config) *Service {
	client := solpkg.NewClient(cfg)
	s := &Service{
		cfg:     cfg,
		solana:  client,
		jupiter: jupiter.NewClient(cfg),
		fetcher: snapshot.NewFetcher(cfg, client),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.LoadAddressLookupTables(ctx, cfg); err != nil {
		logx.Warn("orchestrator", "address lookup tables load failed", "err", err)
	}
	return s
}

type QuoteInput struct {
	InputMint        string
	OutputMint       string
	InputAmount      string
	InputAmountRaw   string // optional exact raw units (lamports / token base units)
	SlippageBPS      int
	UserPubkey       string
	RecipientPubkey  string
	PriorityTier     string
	InputSettlement  string // native_sol | wsol_spl | spl (optional)
	OutputSettlement string // native_sol | wsol_spl | spl (optional)
}

type QuoteResult struct {
	Source                string                  `json:"source"`
	PairClass             string                  `json:"pairClass"`
	Route                 []route.Leg             `json:"route"`
	Quote                 QuoteSummary            `json:"quote"`
	Snapshot              SnapshotMeta            `json:"snapshot"`
	Build                 *BuildResult            `json:"build,omitempty"`
	BuildSkippedReason    string                  `json:"buildSkippedReason,omitempty"`
	BuildError            string                  `json:"buildError,omitempty"`
	Builds                map[string]*BuildResult `json:"builds,omitempty"`
	Capabilities          map[string]Capability   `json:"capabilities,omitempty"`
	SettlementFullBalance bool                    `json:"settlementFullBalance,omitempty"`
	SettlementModes       []string                `json:"settlementModes,omitempty"`
}

type QuoteSummary struct {
	InputMint         string `json:"inputMint"`
	OutputMint        string `json:"outputMint"`
	InputAmount       string `json:"inputAmount"`
	OutputAmount      string `json:"outputAmount,omitempty"`
	MinOutputAmount   string `json:"minOutputAmount,omitempty"`
	GrossOutputAmount string `json:"grossOutputAmount,omitempty"`
	ServiceFeeRaw     string `json:"serviceFeeRaw,omitempty"`
	ServiceFeeBasis   string `json:"serviceFeeBasis,omitempty"`
	PriceImpact       string `json:"priceImpact,omitempty"`
}

type SnapshotMeta struct {
	AccountCount int    `json:"accountCount"`
	Commitment   string `json:"commitment"`
}

func (s *Service) Quote(ctx context.Context, in QuoteInput) (*QuoteResult, error) {
	start := time.Now()
	if in.InputMint == "" || in.OutputMint == "" || in.InputAmount == "" {
		return nil, fmt.Errorf("inputMint, outputMint, and inputAmount are required")
	}
	if in.UserPubkey == "" {
		return nil, fmt.Errorf("userPubkey is required")
	}
	if in.SlippageBPS == 0 {
		in.SlippageBPS = s.cfg.Quote.DefaultSlippageBPS
	}
	in.InputSettlement = NormalizeSettlement(in.InputMint, in.InputSettlement, s.cfg.Quotes.WSOLMint)
	in.OutputSettlement = NormalizeSettlement(in.OutputMint, in.OutputSettlement, s.cfg.Quotes.WSOLMint)
	recipient := in.RecipientPubkey
	if recipient == "" {
		recipient = in.UserPubkey
	}

	if in.InputMint == s.cfg.Quotes.WSOLMint && in.OutputMint == s.cfg.Quotes.WSOLMint {
		if in.InputSettlement == in.OutputSettlement {
			return nil, fmt.Errorf("WSOL settlement unchanged; choose native SOL or WSOL")
		}
		if route.IsSOLSettlementConvert(in.InputMint, in.OutputMint, in.InputSettlement, in.OutputSettlement, s.cfg.Quotes.WSOLMint) {
			return s.quoteSOLSettlement(ctx, in, start)
		}
	}

	pairClass := route.ClassifyPair(in.InputMint, in.OutputMint, s.cfg.IsQuoteMint)
	logx.Info("quote", "start",
		"pairClass", pairClass.String(),
		"inputMint", in.InputMint,
		"outputMint", in.OutputMint,
		"inputAmount", in.InputAmount,
		"slippageBps", in.SlippageBPS,
	)
	var bridgePool *bridge.DiscoveredPool
	var legs []route.Leg
	source := "launchpad"

	switch pairClass {
	case route.PairQuoteSwap:
		source = "quote_swap"
		inDec := quoteDecimals(s.cfg, in.InputMint)
		inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, inDec)
		if err != nil {
			return nil, err
		}
		pool, err := s.jupiter.DiscoverSingleHop(ctx, jupiter.DiscoverRequest{
			InputMint:   in.InputMint,
			OutputMint:  in.OutputMint,
			Amount:      strconv.FormatUint(inputRaw, 10),
			SlippageBPS: in.SlippageBPS,
		})
		if err != nil {
			return nil, err
		}
		bridgePool = pool
		legs = []route.Leg{{
			Kind:       route.LegQuoteBridge,
			InputMint:  in.InputMint,
			OutputMint: in.OutputMint,
			PoolID:     pool.PoolID,
			PoolType:   string(pool.PoolType),
		}}
	default:
		legs = []route.Leg{{
			Kind:       route.LegLaunchpad,
			InputMint:  in.InputMint,
			OutputMint: in.OutputMint,
		}}
	}

	plan, err := snapshot.BuildFetchPlan(s.cfg, snapshot.FetchPlanInput{
		PairClass:  pairClass,
		InputMint:  in.InputMint,
		OutputMint: in.OutputMint,
		UserPubkey: in.UserPubkey,
		Recipient:  recipient,
		BridgePool: bridgePool,
	})
	if err != nil {
		logx.Error("quote", "fetch plan failed", "err", err, "ms", logx.Since(start))
		return nil, err
	}
	logx.Debug("quote", "fetch plan",
		"accountCount", len(plan.Keys),
		"hasBridgePool", bridgePool != nil,
	)

	snapStart := time.Now()
	chainSnap, err := s.fetcher.Fetch(ctx, plan)
	if err != nil {
		logx.Error("quote", "snapshot fetch failed", "err", err, "accounts", len(plan.Keys), "ms", logx.Since(snapStart))
		return nil, err
	}
	logx.Info("quote", "snapshot ok", "accounts", len(plan.Keys), "ms", logx.Since(snapStart))

	result := &QuoteResult{
		Source:    source,
		PairClass: pairClass.String(),
		Route:     legs,
		Snapshot: SnapshotMeta{
			AccountCount: len(plan.Keys),
			Commitment:   s.cfg.Snapshot.Commitment,
		},
		Quote: QuoteSummary{
			InputMint:   in.InputMint,
			OutputMint:  in.OutputMint,
			InputAmount: in.InputAmount,
		},
	}

	if bridgePool != nil {
		out, err := strconv.ParseUint(bridgePool.OutAmount, 10, 64)
		if err == nil {
			result.Quote.OutputAmount = bridgePool.OutAmount
			result.Quote.MinOutputAmount = strconv.FormatUint(util.MinOut(out, in.SlippageBPS), 10)
		} else {
			result.Quote.OutputAmount = bridgePool.OutAmount
		}
		result.Quote.PriceImpact = bridgePool.PriceImpact

		if pairClass == route.PairQuoteSwap {
			variants, err := s.buildQuoteSwap(ctx, in, bridgePool, chainSnap.Accounts)
			if err != nil {
				result.BuildSkippedReason = "build_error"
				result.BuildError = err.Error()
			} else {
				applyVariantsToQuote(result, variants)
			}
		}
	}

	if pairClass != route.PairQuoteSwap {
		baseMintStr := plan.Meta.InputMint
		if s.cfg.IsQuoteMint(baseMintStr) {
			baseMintStr = plan.Meta.OutputMint
		}
		baseMint, err := solpkg.ParsePubkey(baseMintStr)
		if err != nil {
			return nil, err
		}
		detection, err := snapshot.DetectLaunchpadVenue(s.cfg, chainSnap, baseMint)
		if err != nil {
			return nil, err
		}
		if len(result.Route) > 0 {
			result.Route[0].Venue = detection.Venue.String()
		}

		switch detection.Venue {
		case venue.IDPumpfun:
			qNative, err := pumpfun.QNativeFromAccounts(s.cfg, chainSnap.Accounts, baseMint)
			if err != nil {
				return nil, err
			}
			detection.QNative = qNative

			planned := plannedLaunchpadRoute(in, pairClass, qNative, s.cfg.IsQuoteMint)
			logx.Info("quote", "route planned",
				"qNative", qNative,
				"hopCount", planned.HopCount,
				"venue", detection.Venue.String(),
				"baseMint", baseMint.String(),
			)
			for i, leg := range planned.Legs {
				logx.Debug("quote", "planned leg",
					"index", i,
					"kind", leg.Kind,
					"in", leg.InputMint,
					"out", leg.OutputMint,
				)
			}

			var multiBridgePool *bridge.DiscoveredPool
			if route.HasBridgeLeg(planned) {
				bridgeStart := time.Now()
				multiBridgePool, err = s.discoverBridgeForPlanned(ctx, in, pairClass, planned, chainSnap, baseMint)
				if err != nil {
					logx.Error("quote", "bridge discover failed",
						"err", err,
						"ms", logx.Since(bridgeStart),
						"qNative", qNative,
					)
					return nil, err
				}
				logx.Info("quote", "bridge discover ok",
					"poolId", multiBridgePool.PoolID,
					"poolType", multiBridgePool.PoolType,
					"ms", logx.Since(bridgeStart),
				)
			}

			if pairClass == route.PairSwapLaunchpad {
				mintB, err := solpkg.ParsePubkey(in.OutputMint)
				if err != nil {
					return nil, err
				}
				detB, err := snapshot.DetectLaunchpadVenue(s.cfg, chainSnap, mintB)
				if err != nil {
					return nil, err
				}
				if detB.Venue != detection.Venue {
					return nil, fmt.Errorf("swap requires both tokens on the same venue (%s vs %s)", detection.Venue, detB.Venue)
				}
				qB, err := pumpfun.QNativeFromAccounts(s.cfg, chainSnap.Accounts, mintB)
				if err != nil {
					return nil, err
				}
				if qB != qNative {
					return nil, fmt.Errorf("swap requires same pool quote (%s vs %s); cross-quote needs bridge", qNative, qB)
				}
			}

			quoteOutcome, err := s.quotePumpLaunchpad(in, pairClass, planned, multiBridgePool, chainSnap, baseMint)
			if err != nil {
				return nil, err
			}
			applyLaunchpadQuote(&result.Quote, quoteOutcome)

			poolID, poolType := "", ""
			if multiBridgePool != nil {
				poolID = multiBridgePool.PoolID
				poolType = string(multiBridgePool.PoolType)
			}
			result.Route = legsFromPlanned(planned, detection.Venue.String(), poolID, poolType)

			variants, err := s.buildPumpFromPlanned(ctx, in, pairClass, planned, multiBridgePool, chainSnap.Accounts, detection, baseMint)
			if err != nil {
				logx.Warn("quote", "build failed", "err", err, "hopCount", planned.HopCount)
				result.BuildSkippedReason = "build_error"
				result.BuildError = err.Error()
			} else {
				applyVariantsToQuote(result, variants)
				if result.Build != nil {
					logx.Info("quote", "build ok",
						"txBytes", result.Build.TransactionSizeBytes,
						"hopCount", planned.HopCount,
						"variants", len(variants.Builds),
					)
				}
			}
		}
	}

	logx.Info("quote", "done",
		"pairClass", pairClass.String(),
		"hops", len(result.Route),
		"hasBuild", result.Build != nil,
		"ms", logx.Since(start),
	)
	return result, nil
}

func (s *Service) Health(ctx context.Context) error {
	return s.solana.Ping(ctx)
}
