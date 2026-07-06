package ifx

import (
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/ifx-run/ifx/go-sdk/expr"
	"github.com/ifx-run/ifx/go-sdk/structuredcpi"
	"github.com/ifx-run/ifx/go-sdk/typed"
)

// PlanPumpSellWithSOLFee builds sell instructions with Ifx let(quoteDelta) + patched platform fee.
// Requires ixReset — only use when Ifx orchestration is needed.
func PlanPumpSellWithSOLFee(cfg *config.Config, params pumpfun.BuildParams, serviceFeeBPS uint16) ([]solana.Instruction, error) {
	return planPumpSellSOL(cfg, params, serviceFeeBPS, nil, 0)
}

// PlanPumpSellSponsored builds sell + fee + sponsor repay from user SOL proceeds.
func PlanPumpSellSponsored(cfg *config.Config, params pumpfun.BuildParams, serviceFeeBPS uint16, repay SponsoredRepayParams, fixedRepayCost uint64) ([]solana.Instruction, error) {
	return planPumpSellSOL(cfg, params, serviceFeeBPS, &repay, fixedRepayCost)
}

func planPumpSellSOL(cfg *config.Config, params pumpfun.BuildParams, serviceFeeBPS uint16, repay *SponsoredRepayParams, fixedRepayCost uint64) ([]solana.Instruction, error) {
	if serviceFeeBPS == 0 && repay == nil {
		return pumpfun.BuildSellCoreInstructions(params)
	}
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}

	core, err := pumpfun.BuildSellCoreInstructions(params)
	if err != nil {
		return nil, err
	}

	out := []solana.Instruction{s.IxReset()}

	baseline := s.LetBuilder()
	userBefore, err := baseline.Lamports(params.User)
	if err != nil {
		return nil, err
	}
	baselineIx, err := baseline.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, baselineIx)
	out = append(out, core...)

	if serviceFeeBPS == 0 {
		if repay == nil {
			return out, nil
		}
		var zero typed.ScratchValue
		out, zero, err = LetZeroU64(s, out)
		if err != nil {
			return nil, err
		}
		return AppendSponsoredRepay(s, out, *repay, userBefore, zero)
	}

	postSell := s.LetBuilder()
	userAfter, err := postSell.Lamports(params.User)
	if err != nil {
		return nil, err
	}
	quoteDelta, err := postSell.LetEval(expr.Sub(
		expr.Ref(userAfter.Index),
		expr.Ref(userBefore.Index),
	))
	if err != nil {
		return nil, err
	}
	bpsConst, err := postSell.LetConstU64(uint64(serviceFeeBPS))
	if err != nil {
		return nil, err
	}
	fee, err := postSell.LetEval(expr.BpsMulFloor(
		expr.Ref(quoteDelta.Index),
		expr.Ref(bpsConst.Index),
	))
	if err != nil {
		return nil, err
	}
	postSellIx, err := postSell.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, postSellIx)

	feeBuilt, err := structuredcpi.StructuredCpi(
		system.NewTransferInstruction(0, params.User, params.PlatformFeePubkey).Build(),
		structuredcpi.StructuredCpiPatch.SystemTransfer(structuredcpi.AsFrameValue(fee)),
	)
	if err != nil {
		return nil, err
	}
	feeXfer, err := feeBuilt.Build(nil)
	if err != nil {
		return nil, err
	}
	feeCpi, err := s.IxCpi(feeXfer)
	if err != nil {
		return nil, err
	}
	out = append(out, feeCpi)

	if repay == nil {
		return out, nil
	}
	repay.FixedCostLamports = fixedRepayCost
	var zero typed.ScratchValue
	out, zero, err = LetZeroU64(s, out)
	if err != nil {
		return nil, err
	}
	return AppendSponsoredRepay(s, out, *repay, userBefore, zero)
}

// NeedsIfxForPumpSell reports whether sell path requires Ifx frame instructions.
func NeedsIfxForPumpSell(serviceFeeBPS uint16) bool {
	return serviceFeeBPS > 0
}
