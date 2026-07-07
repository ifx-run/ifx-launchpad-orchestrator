package ifx

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx/go-sdk/expr"
	"github.com/ifx-run/ifx/go-sdk/scratch"
	"github.com/ifx-run/ifx/go-sdk/structuredcpi"
	"github.com/ifx-run/ifx/go-sdk/typed"
)

// QuoteBridgeParams wires a single quote-bridge swap with sponsor repay from WSOL output.
type QuoteBridgeParams struct {
	BridgeSwap   solana.Instruction
	UnwrapWSOL   *solana.Instruction // CloseAccount when output is native SOL
	WSOLATA      solana.PublicKey
	User         solana.PublicKey
	TokenProgram solana.PublicKey
	// RepayPartial: sync + patched UnwrapLamports(settle) and keep WSOL ATA (output wsol_spl).
	// Otherwise UnwrapWSOL must be CloseAccount (output native SOL).
	RepayPartial bool
}

// PlanQuoteBridgeSponsored repays sponsor from SOL proceeds after bridge → unwrap.
func PlanQuoteBridgeSponsored(
	cfg *config.Config,
	p QuoteBridgeParams,
	repay SponsoredRepayParams,
	fixedRepay uint64,
	sponsorPayer solana.PublicKey,
	ataSpecs []ATASetupSpec,
) ([]solana.Instruction, error) {
	if p.RepayPartial {
		return planQuoteBridgeSponsoredPartial(cfg, p, repay, fixedRepay, sponsorPayer, ataSpecs)
	}
	if p.UnwrapWSOL == nil {
		return nil, fmt.Errorf("sponsored quote bridge requires WSOL unwrap for SOL repay")
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

	out, userBefore, err := LetLamportsBaseline(s, out, repay.User)
	if err != nil {
		return nil, err
	}

	out = append(out, p.BridgeSwap)
	out = append(out, *p.UnwrapWSOL)

	repay.FixedCostLamports = fixedRepay
	return AppendSponsoredRepay(s, out, repay, userBefore, ataCost)
}

func planQuoteBridgeSponsoredPartial(
	cfg *config.Config,
	p QuoteBridgeParams,
	repay SponsoredRepayParams,
	fixedRepay uint64,
	sponsorPayer solana.PublicKey,
	ataSpecs []ATASetupSpec,
) ([]solana.Instruction, error) {
	if p.WSOLATA.IsZero() || p.User.IsZero() {
		return nil, fmt.Errorf("partial repay requires WSOL ATA and user")
	}
	tp := p.TokenProgram
	if tp.IsZero() {
		tp = solana.TokenProgramID
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

	out = append(out, p.BridgeSwap)

	settle, err := letSettleScratch(s, &out, ataCost, fixedRepay)
	if err != nil {
		return nil, err
	}

	postSwap := s.LetBuilder()
	wsolAfter, err := postSwap.SplTokenAmount(p.WSOLATA)
	if err != nil {
		return nil, err
	}
	postSwapIx, err := postSwap.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, postSwapIx)

	assertIx, err := s.IxAssert(expr.Ge(
		expr.Ref(wsolAfter.Index),
		expr.Ref(settle.Index),
	))
	if err != nil {
		return nil, err
	}
	out = append(out, assertIx)

	out = append(out, solpkg.SyncNativeInstruction(p.WSOLATA, tp))

	var zero uint64
	unwrapTpl := solpkg.UnwrapLamportsInstruction(p.WSOLATA, repay.RepayTo, p.User, tp, &zero)
	unwrapBuilt, err := structuredcpi.StructuredCpi(
		unwrapTpl,
		structuredcpi.StructuredCpiPatch.TokenUnwrapLamportsAmount(structuredcpi.AsFrameValue(settle)),
	)
	if err != nil {
		return nil, err
	}
	unwrapXfer, err := unwrapBuilt.Build(nil)
	if err != nil {
		return nil, err
	}
	unwrapCpi, err := s.IxCpi(unwrapXfer)
	if err != nil {
		return nil, err
	}
	out = append(out, unwrapCpi)
	return out, nil
}

func letSettleScratch(s *scratch.FrameScratch, out *[]solana.Instruction, ataCost typed.ScratchValue, fixedRepay uint64) (typed.ScratchValue, error) {
	b := s.LetBuilder()
	fixedConst, err := b.LetConstU64(fixedRepay)
	if err != nil {
		return typed.ScratchValue{}, err
	}
	settle, err := b.LetEval(expr.Add(
		expr.Ref(ataCost.Index),
		expr.Ref(fixedConst.Index),
	))
	if err != nil {
		return typed.ScratchValue{}, err
	}
	bIx, err := b.BuildIx()
	if err != nil {
		return typed.ScratchValue{}, err
	}
	*out = append(*out, bIx)
	return settle, nil
}

// NeedsIfxForQuoteBridgeSponsored reports whether the route must be Ifx-orchestrated for sponsored repay.
func NeedsIfxForQuoteBridgeSponsored(p QuoteBridgeParams) bool {
	return p.RepayPartial || p.UnwrapWSOL != nil
}
