package ifx

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/ifx-run/ifx/go-sdk/expr"
	"github.com/ifx-run/ifx/go-sdk/scratch"
	"github.com/ifx-run/ifx/go-sdk/structuredcpi"
	"github.com/ifx-run/ifx/go-sdk/typed"
)

// SponsoredRepayParams configures sponsor repayment from user SOL proceeds.
type SponsoredRepayParams struct {
	User              solana.PublicKey
	RepayTo           solana.PublicKey // gas treasury; may differ from fee payer
	FixedCostLamports uint64           // basic + priority + tip (off-chain estimate)
}

// AppendSponsoredRepay adds let(post) → assert → patched repay transfer to RepayTo.
// userLamportsBefore is the ScratchValue from the opening let(user lamports).
// ataCost may be zero when no sponsor-paid ATA rent is measured on-chain.
func AppendSponsoredRepay(
	s *scratch.FrameScratch,
	out []solana.Instruction,
	p SponsoredRepayParams,
	userLamportsBefore typed.ScratchValue,
	ataCost typed.ScratchValue,
) ([]solana.Instruction, error) {
	post := s.LetBuilder()
	userAfter, err := post.Lamports(p.User)
	if err != nil {
		return nil, err
	}
	proceeds, err := post.LetEval(expr.Sub(
		expr.Ref(userAfter.Index),
		expr.Ref(userLamportsBefore.Index),
	))
	if err != nil {
		return nil, err
	}
	settle, err := post.LetEval(expr.Add(
		expr.Ref(ataCost.Index),
		expr.U64(p.FixedCostLamports),
	))
	if err != nil {
		return nil, err
	}
	postIx, err := post.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, postIx)

	assertIx, err := s.IxAssert(expr.Ge(
		expr.Ref(proceeds.Index),
		expr.Ref(settle.Index),
	))
	if err != nil {
		return nil, err
	}
	out = append(out, assertIx)

	repayBuilt, err := structuredcpi.StructuredCpi(
		system.NewTransferInstruction(0, p.User, p.RepayTo).Build(),
		structuredcpi.StructuredCpiPatch.SystemTransfer(structuredcpi.AsFrameValue(settle)),
	)
	if err != nil {
		return nil, err
	}
	repayXfer, err := repayBuilt.Build(nil)
	if err != nil {
		return nil, err
	}
	repayCpi, err := s.IxCpi(repayXfer)
	if err != nil {
		return nil, err
	}
	out = append(out, repayCpi)
	return out, nil
}

// AppendRepayDeductNet asserts proceeds covers repay, transfers settle to RepayTo, returns net for downstream hops.
func AppendRepayDeductNet(
	s *scratch.FrameScratch,
	out []solana.Instruction,
	p SponsoredRepayParams,
	proceeds typed.ScratchValue,
	ataCost typed.ScratchValue,
	fixedRepay uint64,
) ([]solana.Instruction, typed.ScratchValue, error) {
	b := s.LetBuilder()
	fixedConst, err := b.LetConstU64(fixedRepay)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	settle, err := b.LetEval(expr.Add(
		expr.Ref(ataCost.Index),
		expr.Ref(fixedConst.Index),
	))
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	net, err := b.LetEval(expr.Sub(
		expr.Ref(proceeds.Index),
		expr.Ref(settle.Index),
	))
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	bIx, err := b.BuildIx()
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	out = append(out, bIx)

	assertIx, err := s.IxAssert(expr.Ge(
		expr.Ref(proceeds.Index),
		expr.Ref(settle.Index),
	))
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	out = append(out, assertIx)

	repayBuilt, err := structuredcpi.StructuredCpi(
		system.NewTransferInstruction(0, p.User, p.RepayTo).Build(),
		structuredcpi.StructuredCpiPatch.SystemTransfer(structuredcpi.AsFrameValue(settle)),
	)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	repayXfer, err := repayBuilt.Build(nil)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	repayCpi, err := s.IxCpi(repayXfer)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	out = append(out, repayCpi)
	return out, net, nil
}

// LetLamportsBaseline records an account's lamports before sponsor-paid setup.
func LetLamportsBaseline(s *scratch.FrameScratch, out []solana.Instruction, acct solana.PublicKey) ([]solana.Instruction, typed.ScratchValue, error) {
	b := s.LetBuilder()
	v, err := b.Lamports(acct)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	ix, err := b.BuildIx()
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	return append(out, ix), v, nil
}

// LetATARentDelta measures lamports deposited into an ATA between before/after snapshots.
func LetATARentDelta(s *scratch.FrameScratch, out []solana.Instruction, ata solana.PublicKey, before typed.ScratchValue) ([]solana.Instruction, typed.ScratchValue, error) {
	b := s.LetBuilder()
	after, err := b.Lamports(ata)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	cost, err := b.LetEval(expr.Sub(
		expr.Ref(after.Index),
		expr.Ref(before.Index),
	))
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	ix, err := b.BuildIx()
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	return append(out, ix), cost, nil
}

// LetZeroU64 binds a zero u64 scratch slot (e.g. when no ATA rent is measured).
func LetZeroU64(s *scratch.FrameScratch, out []solana.Instruction) ([]solana.Instruction, typed.ScratchValue, error) {
	b := s.LetBuilder()
	z, err := b.LetConstU64(0)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	ix, err := b.BuildIx()
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	return append(out, ix), z, nil
}
