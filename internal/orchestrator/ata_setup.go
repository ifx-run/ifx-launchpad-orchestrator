package orchestrator

import (
	"fmt"

	ifxpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/ifx"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

type ataKey struct {
	owner solana.PublicKey
	mint  solana.PublicKey
	tp    solana.PublicKey
}

// ataSetup collects CreateIdempotent instructions, one per (owner, mint, token program).
type ataSetup struct {
	seen map[ataKey]struct{}
	keys []ataKey
}

func newATASetup() *ataSetup {
	return &ataSetup{seen: make(map[ataKey]struct{})}
}

func (a *ataSetup) ensure(payer, owner, mint, tokenProgram solana.PublicKey) error {
	k := ataKey{owner: owner, mint: mint, tp: tokenProgram}
	if _, ok := a.seen[k]; ok {
		return nil
	}
	a.seen[k] = struct{}{}
	a.keys = append(a.keys, k)
	_ = payer // payer is chosen when instructions are materialized
	return nil
}

func (a *ataSetup) instructions(payer solana.PublicKey) ([]solana.Instruction, error) {
	out := make([]solana.Instruction, 0, len(a.keys))
	for _, k := range a.keys {
		ix, err := createATAIdempotent(payer, k.owner, k.mint, k.tp)
		if err != nil {
			return nil, err
		}
		out = append(out, ix)
	}
	return out, nil
}

func (a *ataSetup) appendTo(dst *[]solana.Instruction, payer solana.PublicKey) error {
	ixs, err := a.instructions(payer)
	if err != nil {
		return err
	}
	*dst = append(*dst, ixs...)
	return nil
}

func (a *ataSetup) count() int {
	return len(a.keys)
}

func (a *ataSetup) ifxSpecs() []ifxpkg.ATASetupSpec {
	specs := make([]ifxpkg.ATASetupSpec, len(a.keys))
	for i, k := range a.keys {
		specs[i] = ifxpkg.ATASetupSpec{
			Owner:        k.owner,
			Mint:         k.mint,
			TokenProgram: k.tp,
		}
	}
	return specs
}

func ensurePumpBuyATAs(
	ata *ataSetup,
	user, baseMint, baseTP, quoteMint, quoteTP solana.PublicKey,
	kind pumpfun.QuoteKind,
) error {
	switch kind {
	case pumpfun.QuoteNativeSOL:
		return ata.ensure(user, user, baseMint, baseTP)
	case pumpfun.QuoteSPL:
		if err := ata.ensure(user, user, baseMint, baseTP); err != nil {
			return err
		}
		return ata.ensure(user, user, quoteMint, quoteTP)
	default:
		return fmt.Errorf("unsupported quote kind")
	}
}

func appendWrapSOLDeposit(
	ixs *[]solana.Instruction,
	payer, owner, wsolMint, tokenProgram solana.PublicKey,
	lamports uint64,
) error {
	pair, err := solpkg.DeriveATAPair(owner, wsolMint)
	if err != nil {
		return err
	}
	ata := solpkg.SelectATA(pair, tokenProgram)
	*ixs = append(*ixs, system.NewTransferInstruction(lamports, payer, ata).Build())
	*ixs = append(*ixs, solpkg.SyncNativeInstruction(ata, tokenProgram))
	return nil
}

func createATAIdempotent(payer, owner, mint, tokenProgram solana.PublicKey) (solana.Instruction, error) {
	ata, _, err := solana.FindAssociatedTokenAddress(owner, mint)
	if tokenProgram.String() == solpkg.Token2022ProgramID {
		ata, _, err = solana.FindProgramAddress(
			[][]byte{owner.Bytes(), tokenProgram.Bytes(), mint.Bytes()},
			solana.SPLAssociatedTokenAccountProgramID,
		)
	}
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
