package ifx

import (
	"github.com/gagliardetto/solana-go"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx/go-sdk/expr"
	"github.com/ifx-run/ifx/go-sdk/scratch"
	"github.com/ifx-run/ifx/go-sdk/typed"
)

// ATASetupSpec describes a user-owned ATA that sponsor may create and fund.
type ATASetupSpec struct {
	Owner        solana.PublicKey
	Mint         solana.PublicKey
	TokenProgram solana.PublicKey
}

func ataAddress(owner, mint, tokenProgram solana.PublicKey) (solana.PublicKey, error) {
	if tokenProgram.String() == solpkg.Token2022ProgramID {
		ata, _, err := solana.FindProgramAddress(
			[][]byte{owner.Bytes(), tokenProgram.Bytes(), mint.Bytes()},
			solana.SPLAssociatedTokenAccountProgramID,
		)
		return ata, err
	}
	ata, _, err := solana.FindAssociatedTokenAddress(owner, mint)
	return ata, err
}

func createATAIdempotentIx(payer, owner, mint, tokenProgram solana.PublicKey) (solana.Instruction, error) {
	ata, err := ataAddress(owner, mint, tokenProgram)
	if err != nil {
		return nil, err
	}
	return solana.NewInstruction(
		solana.SPLAssociatedTokenAccountProgramID,
		solana.AccountMetaSlice{
			{PublicKey: payer, IsWritable: true, IsSigner: true},
			{PublicKey: ata, IsWritable: true, IsSigner: false},
			{PublicKey: owner, IsWritable: false, IsSigner: false},
			{PublicKey: mint, IsWritable: false, IsSigner: false},
			{PublicKey: solana.SystemProgramID, IsWritable: false, IsSigner: false},
			{PublicKey: tokenProgram, IsWritable: false, IsSigner: false},
		},
		[]byte{1},
	), nil
}

// AppendSponsorATACreates runs idempotent ATA creates (sponsor-paid) and returns on-chain rent sum.
// Rent is measured as sponsor lamports spent (before − after), not per-ATA reads — uninitialized
// ATA addresses cannot be loaded in a let before create.
func AppendSponsorATACreates(
	s *scratch.FrameScratch,
	out []solana.Instruction,
	sponsorPayer solana.PublicKey,
	specs []ATASetupSpec,
) ([]solana.Instruction, typed.ScratchValue, error) {
	if len(specs) == 0 {
		return LetZeroU64(s, out)
	}

	out, sponsorBefore, err := LetLamportsBaseline(s, out, sponsorPayer)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}

	for _, spec := range specs {
		createIx, err := createATAIdempotentIx(sponsorPayer, spec.Owner, spec.Mint, spec.TokenProgram)
		if err != nil {
			return out, typed.ScratchValue{}, err
		}
		out = append(out, createIx)
	}

	b := s.LetBuilder()
	sponsorAfter, err := b.Lamports(sponsorPayer)
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	rent, err := b.LetEval(expr.Sub(
		expr.Ref(sponsorBefore.Index),
		expr.Ref(sponsorAfter.Index),
	))
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	ix, err := b.BuildIx()
	if err != nil {
		return out, typed.ScratchValue{}, err
	}
	return append(out, ix), rent, nil
}
