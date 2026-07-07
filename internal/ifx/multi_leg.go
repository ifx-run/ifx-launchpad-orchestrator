package ifx

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/ifx-run/ifx/go-sdk/codec"
	"github.com/ifx-run/ifx/go-sdk/expr"
	"github.com/ifx-run/ifx/go-sdk/patch"
	"github.com/ifx-run/ifx/go-sdk/patchedcpi"
	"github.com/ifx-run/ifx/go-sdk/scratch"
	"github.com/ifx-run/ifx/go-sdk/structuredcpi"
	"github.com/ifx-run/ifx/go-sdk/typed"
)

// NeedsIfxForMultiLeg reports whether a multi-hop route must be Ifx-orchestrated.
// True for every 2-hop launchpad route: hop chaining (let + patch) is required regardless
// of where platform fee is taken (input before bridge, or output delta after sell).
func NeedsIfxForMultiLeg() bool {
	return true
}

// BridgeThenPumpBuyParams wires bridge hop1 → let(quoteDelta) → patched pump buy.
type BridgeThenPumpBuyParams struct {
	BridgeSwap      solana.Instruction
	MeasureQuoteATA solana.PublicKey
	BuyTemplate     solana.Instruction
	UnwrapWSOL      *solana.Instruction
}

// PlanBridgeThenPumpBuy returns reset → let(before) → bridge → let(delta) → [unwrap] → patched buy.
func PlanBridgeThenPumpBuy(cfg *config.Config, p BridgeThenPumpBuyParams) ([]solana.Instruction, error) {
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out := []solana.Instruction{s.IxReset()}

	baseline := s.LetBuilder()
	quoteBefore, err := baseline.SplTokenAmount(p.MeasureQuoteATA)
	if err != nil {
		return nil, err
	}
	baselineIx, err := baseline.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, baselineIx)

	out = append(out, p.BridgeSwap)

	postBridge := s.LetBuilder()
	quoteAfter, err := postBridge.SplTokenAmount(p.MeasureQuoteATA)
	if err != nil {
		return nil, err
	}
	quoteDelta, err := postBridge.LetEval(expr.Sub(
		expr.Ref(quoteAfter.Index),
		expr.Ref(quoteBefore.Index),
	))
	if err != nil {
		return nil, err
	}
	postBridgeIx, err := postBridge.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, postBridgeIx)

	if p.UnwrapWSOL != nil {
		out = append(out, *p.UnwrapWSOL)
	}

	buyCpi, err := rawCpiIxScratch(s, p.BuyTemplate, patch.RawCpiPatch(pumpfun.BuySpendableQuoteInOffset, quoteDelta))
	if err != nil {
		return nil, err
	}
	out = append(out, buyCpi)
	return out, nil
}

// PlanBridgeThenPumpBuySponsored repays sponsor from bridge quoteDelta (WSOL/lamports) then patched buy.
func PlanBridgeThenPumpBuySponsored(cfg *config.Config, p BridgeThenPumpBuyParams, repay SponsoredRepayParams, fixedRepay uint64, sponsorPayer solana.PublicKey, ataSpecs []ATASetupSpec) ([]solana.Instruction, error) {
	if p.UnwrapWSOL == nil {
		return nil, fmt.Errorf("sponsored bridge→buy requires WSOL unwrap for SOL repay")
	}
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out := []solana.Instruction{s.IxReset()}

	out, ataCost, err := AppendSponsorATACreates(s, out, sponsorPayer, ataSpecs)
	if err != nil {
		return nil, err
	}

	baseline := s.LetBuilder()
	quoteBefore, err := baseline.SplTokenAmount(p.MeasureQuoteATA)
	if err != nil {
		return nil, err
	}
	baselineIx, err := baseline.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, baselineIx)

	out = append(out, p.BridgeSwap)

	postBridge := s.LetBuilder()
	quoteAfter, err := postBridge.SplTokenAmount(p.MeasureQuoteATA)
	if err != nil {
		return nil, err
	}
	quoteDelta, err := postBridge.LetEval(expr.Sub(
		expr.Ref(quoteAfter.Index),
		expr.Ref(quoteBefore.Index),
	))
	if err != nil {
		return nil, err
	}
	postBridgeIx, err := postBridge.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, postBridgeIx)

	out = append(out, *p.UnwrapWSOL)

	repay.FixedCostLamports = fixedRepay
	out, netBuy, err := AppendRepayDeductNet(s, out, repay, quoteDelta, ataCost, fixedRepay)
	if err != nil {
		return nil, err
	}

	buyCpi, err := rawCpiIxScratch(s, p.BuyTemplate, patch.RawCpiPatch(pumpfun.BuySpendableQuoteInOffset, netBuy))
	if err != nil {
		return nil, err
	}
	out = append(out, buyCpi)
	return out, nil
}

// PlanPumpSellThenBridgeSponsored repays sponsor from sell SOL proceeds then patched bridge.
func PlanPumpSellThenBridgeSponsored(cfg *config.Config, p SellThenBridgeParams, repay SponsoredRepayParams, fixedRepay uint64, sponsorPayer solana.PublicKey, ataSpecs []ATASetupSpec) ([]solana.Instruction, error) {
	if p.QuoteKind != pumpfun.QuoteNativeSOL {
		return nil, fmt.Errorf("sponsored sell→bridge requires SOL quote pool")
	}
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out := []solana.Instruction{s.IxReset()}

	out, ataCost, err := AppendSponsorATACreates(s, out, sponsorPayer, ataSpecs)
	if err != nil {
		return nil, err
	}

	out, netLamports, err := planSellDeltaFeePrefixSOLScratchAfterReset(s, out, p)
	if err != nil {
		return nil, err
	}
	repay.FixedCostLamports = fixedRepay
	out, bridgeIn, err := AppendRepayDeductNet(s, out, repay, netLamports, ataCost, fixedRepay)
	if err != nil {
		return nil, err
	}
	return appendBridgeAfterNetSOL(s, out, p, bridgeIn)
}

func appendBridgeAfterNetSOL(s *scratch.FrameScratch, out []solana.Instruction, p SellThenBridgeParams, bridgeIn typed.ScratchValue) ([]solana.Instruction, error) {
	if p.WrapBeforeBridge {
		wrapXferTpl := system.NewTransferInstruction(0, p.User, p.WSOLATA).Build()
		wrapXfer, err := structuredCpiWire(wrapXferTpl, structuredcpi.StructuredCpiPatch.SystemTransfer(structuredcpi.AsFrameValue(bridgeIn)))
		if err != nil {
			return nil, err
		}
		wrapCpi, err := s.IxCpi(wrapXfer)
		if err != nil {
			return nil, err
		}
		out = append(out, wrapCpi)
		out = append(out, solpkg.SyncNativeInstruction(p.WSOLATA, solana.TokenProgramID))
	}
	bridgeCpi, err := rawCpiIxScratch(s, p.BridgeTemplate, patch.RawCpiPatch(p.BridgeAmountInOffset, bridgeIn))
	if err != nil {
		return nil, err
	}
	out = append(out, bridgeCpi)
	return out, nil
}

// SellThenBridgeParams wires pump sell → fee on delta → patched bridge amount_in.
type SellThenBridgeParams struct {
	QuoteKind            pumpfun.QuoteKind
	SellTemplate         solana.Instruction
	BridgeTemplate       solana.Instruction
	BridgeAmountInOffset uint16
	ServiceFeeBPS        uint16
	User                 solana.PublicKey
	PlatformFeePubkey    solana.PublicKey
	PlatformFeeQuoteATA  solana.PublicKey
	QuoteATA             solana.PublicKey
	QuoteMint            solana.PublicKey
	QuoteTokenProgram    solana.PublicKey
	QuoteDecimals        uint8
	WSOLATA              solana.PublicKey
	WrapBeforeBridge     bool
}

// PlanPumpSellThenBridge returns reset → let(before) → sell → let(delta) → fee → patched bridge [→ wrap].
func PlanPumpSellThenBridge(cfg *config.Config, p SellThenBridgeParams) ([]solana.Instruction, error) {
	switch p.QuoteKind {
	case pumpfun.QuoteNativeSOL:
		return planPumpSellThenBridgeSOL(cfg, p)
	case pumpfun.QuoteSPL:
		return planPumpSellThenBridgeSPL(cfg, p)
	default:
		return nil, fmt.Errorf("unsupported quote kind for sell→bridge")
	}
}

// SellThenBuyParams wires pump sell(A) → fee on delta → patched buy(B).
type SellThenBuyParams struct {
	QuoteKind           pumpfun.QuoteKind
	SellTemplate        solana.Instruction
	BuyTemplate         solana.Instruction
	ServiceFeeBPS       uint16
	User                solana.PublicKey
	PlatformFeePubkey   solana.PublicKey
	PlatformFeeQuoteATA solana.PublicKey
	QuoteATA            solana.PublicKey
	QuoteMint           solana.PublicKey
	QuoteTokenProgram   solana.PublicKey
	QuoteDecimals       uint8
}

// PlanPumpSellThenBuy returns reset → let(before) → sell → let(delta) → fee → patched buy.
func PlanPumpSellThenBuy(cfg *config.Config, p SellThenBuyParams) ([]solana.Instruction, error) {
	feeParams := sellThenBuyFeeParams(p)
	switch p.QuoteKind {
	case pumpfun.QuoteNativeSOL:
		return planPumpSellThenBuySOL(cfg, feeParams, p.BuyTemplate)
	case pumpfun.QuoteSPL:
		return planPumpSellThenBuySPL(cfg, feeParams, p.BuyTemplate)
	default:
		return nil, fmt.Errorf("unsupported quote kind for sell→buy")
	}
}

// PlanPumpSellThenBuySponsored repays sponsor from sell SOL proceeds then patched buy(B).
func PlanPumpSellThenBuySponsored(cfg *config.Config, p SellThenBuyParams, repay SponsoredRepayParams, fixedRepay uint64, sponsorPayer solana.PublicKey, ataSpecs []ATASetupSpec) ([]solana.Instruction, error) {
	if p.QuoteKind != pumpfun.QuoteNativeSOL {
		return nil, fmt.Errorf("sponsored sell→buy requires SOL quote pool")
	}
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out := []solana.Instruction{s.IxReset()}

	out, ataCost, err := AppendSponsorATACreates(s, out, sponsorPayer, ataSpecs)
	if err != nil {
		return nil, err
	}

	out, netLamports, err := planSellDeltaFeePrefixSOLScratchAfterReset(s, out, sellThenBuyFeeParams(p))
	if err != nil {
		return nil, err
	}
	repay.FixedCostLamports = fixedRepay
	out, buyIn, err := AppendRepayDeductNet(s, out, repay, netLamports, ataCost, fixedRepay)
	if err != nil {
		return nil, err
	}
	buyCpi, err := rawCpiIxScratch(s, p.BuyTemplate, patch.RawCpiPatch(pumpfun.BuySpendableQuoteInOffset, buyIn))
	if err != nil {
		return nil, err
	}
	out = append(out, buyCpi)
	return out, nil
}

func sellThenBuyFeeParams(p SellThenBuyParams) SellThenBridgeParams {
	return SellThenBridgeParams{
		QuoteKind:           p.QuoteKind,
		SellTemplate:        p.SellTemplate,
		ServiceFeeBPS:       p.ServiceFeeBPS,
		User:                p.User,
		PlatformFeePubkey:   p.PlatformFeePubkey,
		PlatformFeeQuoteATA: p.PlatformFeeQuoteATA,
		QuoteATA:            p.QuoteATA,
		QuoteMint:           p.QuoteMint,
		QuoteTokenProgram:   p.QuoteTokenProgram,
		QuoteDecimals:       p.QuoteDecimals,
	}
}

func planPumpSellThenBuySPL(cfg *config.Config, p SellThenBridgeParams, buyTemplate solana.Instruction) ([]solana.Instruction, error) {
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out, buyIn, err := planSellDeltaFeePrefixScratch(s, p)
	if err != nil {
		return nil, err
	}
	buyCpi, err := rawCpiIxScratch(s, buyTemplate, patch.RawCpiPatch(pumpfun.BuySpendableQuoteInOffset, buyIn))
	if err != nil {
		return nil, err
	}
	out = append(out, buyCpi)
	return out, nil
}

func planPumpSellThenBuySOL(cfg *config.Config, p SellThenBridgeParams, buyTemplate solana.Instruction) ([]solana.Instruction, error) {
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out, buyIn, err := planSellDeltaFeePrefixSOLScratch(s, p)
	if err != nil {
		return nil, err
	}
	buyCpi, err := rawCpiIxScratch(s, buyTemplate, patch.RawCpiPatch(pumpfun.BuySpendableQuoteInOffset, buyIn))
	if err != nil {
		return nil, err
	}
	out = append(out, buyCpi)
	return out, nil
}

func planPumpSellThenBridgeSPL(cfg *config.Config, p SellThenBridgeParams) ([]solana.Instruction, error) {
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out, bridgeIn, err := planSellDeltaFeePrefixScratch(s, p)
	if err != nil {
		return nil, err
	}
	bridgeCpi, err := rawCpiIxScratch(s, p.BridgeTemplate, patch.RawCpiPatch(p.BridgeAmountInOffset, bridgeIn))
	if err != nil {
		return nil, err
	}
	out = append(out, bridgeCpi)
	return out, nil
}

func planPumpSellThenBridgeSOL(cfg *config.Config, p SellThenBridgeParams) ([]solana.Instruction, error) {
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out, netLamports, err := planSellDeltaFeePrefixSOLScratch(s, p)
	if err != nil {
		return nil, err
	}
	return appendBridgeAfterNetSOL(s, out, p, netLamports)
}

func planSellDeltaFeePrefixScratch(s *scratch.FrameScratch, p SellThenBridgeParams) ([]solana.Instruction, typed.ScratchValue, error) {
	out := []solana.Instruction{s.IxReset()}

	baseline := s.LetBuilder()
	quoteBefore, err := baseline.SplTokenAmount(p.QuoteATA)
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	baselineIx, err := baseline.BuildIx()
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	out = append(out, baselineIx)
	out = append(out, p.SellTemplate)

	postSell := s.LetBuilder()
	quoteAfter, err := postSell.SplTokenAmount(p.QuoteATA)
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	quoteDelta, err := postSell.LetEval(expr.Sub(
		expr.Ref(quoteAfter.Index),
		expr.Ref(quoteBefore.Index),
	))
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	postSellIx, err := postSell.BuildIx()
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	out = append(out, postSellIx)

	net, err := appendServiceFeeAndNet(nil, s, &out, p, quoteDelta)
	return out, net, err
}

func planSellDeltaFeePrefixSOLScratch(s *scratch.FrameScratch, p SellThenBridgeParams) ([]solana.Instruction, typed.ScratchValue, error) {
	out := []solana.Instruction{s.IxReset()}
	return planSellDeltaFeePrefixSOLScratchAfterReset(s, out, p)
}

func planSellDeltaFeePrefixSOLScratchAfterReset(s *scratch.FrameScratch, out []solana.Instruction, p SellThenBridgeParams) ([]solana.Instruction, typed.ScratchValue, error) {
	baseline := s.LetBuilder()
	userBefore, err := baseline.Lamports(p.User)
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	baselineIx, err := baseline.BuildIx()
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	out = append(out, baselineIx)
	out = append(out, p.SellTemplate)

	postSell := s.LetBuilder()
	userAfter, err := postSell.Lamports(p.User)
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	quoteDelta, err := postSell.LetEval(expr.Sub(
		expr.Ref(userAfter.Index),
		expr.Ref(userBefore.Index),
	))
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	postSellIx, err := postSell.BuildIx()
	if err != nil {
		return nil, typed.ScratchValue{}, err
	}
	out = append(out, postSellIx)

	net, err := appendServiceFeeAndNet(nil, s, &out, p, quoteDelta)
	return out, net, err
}

func rawCpiIxScratch(s *scratch.FrameScratch, template solana.Instruction, patches ...codec.RawCpiPatch) (solana.Instruction, error) {
	built, err := patchedcpi.RawCpi(template, patches...).Build(nil)
	if err != nil {
		return nil, err
	}
	return s.IxCpi(built.WireBuild())
}

func appendServiceFeeAndNet(
	_ *config.Config,
	s *scratch.FrameScratch,
	out *[]solana.Instruction,
	p SellThenBridgeParams,
	quoteDelta typed.ScratchValue,
) (typed.ScratchValue, error) {
	if p.ServiceFeeBPS == 0 {
		return quoteDelta, nil
	}
	feeBuilder := s.LetBuilder()
	bpsConst, err := feeBuilder.LetConstU64(uint64(p.ServiceFeeBPS))
	if err != nil {
		return typed.ScratchValue{}, err
	}
	fee, err := feeBuilder.LetEval(expr.BpsMulFloor(
		expr.Ref(quoteDelta.Index),
		expr.Ref(bpsConst.Index),
	))
	if err != nil {
		return typed.ScratchValue{}, err
	}
	net, err := feeBuilder.LetEval(expr.Sub(
		expr.Ref(quoteDelta.Index),
		expr.Ref(fee.Index),
	))
	if err != nil {
		return typed.ScratchValue{}, err
	}
	feeBuilderIx, err := feeBuilder.BuildIx()
	if err != nil {
		return typed.ScratchValue{}, err
	}
	*out = append(*out, feeBuilderIx)

	switch p.QuoteKind {
	case pumpfun.QuoteNativeSOL:
		feeTpl := system.NewTransferInstruction(0, p.User, p.PlatformFeePubkey).Build()
		feeXfer, err := structuredCpiWire(feeTpl, structuredcpi.StructuredCpiPatch.SystemTransfer(structuredcpi.AsFrameValue(fee)))
		if err != nil {
			return typed.ScratchValue{}, err
		}
		feeCpi, err := s.IxCpi(feeXfer)
		if err != nil {
			return typed.ScratchValue{}, err
		}
		*out = append(*out, feeCpi)
	case pumpfun.QuoteSPL:
		feeTpl := token.NewTransferCheckedInstruction(
			0,
			p.QuoteDecimals,
			p.QuoteATA,
			p.QuoteMint,
			p.PlatformFeeQuoteATA,
			p.User,
			[]solana.PublicKey{},
		).Build()
		feeXfer, err := structuredCpiWire(
			feeTpl,
			structuredcpi.StructuredCpiPatch.TokenTransferChecked().AmountOnly(structuredcpi.AsFrameValue(fee), p.QuoteDecimals),
		)
		if err != nil {
			return typed.ScratchValue{}, err
		}
		feeCpi, err := s.IxCpi(feeXfer)
		if err != nil {
			return typed.ScratchValue{}, err
		}
		*out = append(*out, feeCpi)
	default:
		return typed.ScratchValue{}, fmt.Errorf("unsupported quote kind for service fee")
	}
	return net, nil
}

func structuredCpiWire(template solana.Instruction, patch structuredcpi.PatchInput) (codec.WireBuildResult, error) {
	built, err := structuredcpi.StructuredCpi(template, patch)
	if err != nil {
		return codec.WireBuildResult{}, err
	}
	return built.Build(nil)
}
