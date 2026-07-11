package dammv2

import (
	"encoding/binary"

	"github.com/gagliardetto/solana-go"
)

// Mainnet pool authority PDA (seed "pool_authority") — fixed in cp-amm IDL.
var PoolAuthority = solana.MustPublicKeyFromBase58("HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC")

var discSwap2 = [8]byte{65, 75, 63, 76, 235, 91, 91, 136}

const swapModeExactIn uint8 = 0

type SwapParams struct {
	ProgramID       solana.PublicKey
	Payer           solana.PublicKey
	Pool            PoolState
	PoolID          solana.PublicKey
	UserInputATA    solana.PublicKey
	UserOutputATA   solana.PublicKey
	InputMint       solana.PublicKey
	OutputMint      solana.PublicKey
	TokenProgramA   solana.PublicKey
	TokenProgramB   solana.PublicKey
	AmountIn        uint64
	MinAmountOut    uint64
}

func EventAuthority(programID solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("__event_authority")},
		programID,
	)
	return pda, err
}

func BuildSwap2(p SwapParams) (solana.Instruction, error) {
	eventAuth, err := EventAuthority(p.ProgramID)
	if err != nil {
		return nil, err
	}
	side, err := p.Pool.Side(p.InputMint, p.OutputMint, p.TokenProgramA, p.TokenProgramB)
	if err != nil {
		return nil, err
	}
	_ = side

	data := make([]byte, 25)
	copy(data[:8], discSwap2[:])
	binary.LittleEndian.PutUint64(data[8:16], p.AmountIn)
	binary.LittleEndian.PutUint64(data[16:24], p.MinAmountOut)
	data[24] = swapModeExactIn

	// Optional referral: pass program id when unused (Anchor optional account).
	referral := p.ProgramID

	// token_a_program / token_b_program are fixed to pool mint A/B, not swap direction.
	return solana.NewInstruction(p.ProgramID, solana.AccountMetaSlice{
		{PublicKey: PoolAuthority, IsWritable: false, IsSigner: false},
		{PublicKey: p.PoolID, IsWritable: true, IsSigner: false},
		{PublicKey: p.UserInputATA, IsWritable: true, IsSigner: false},
		{PublicKey: p.UserOutputATA, IsWritable: true, IsSigner: false},
		{PublicKey: p.Pool.VaultA, IsWritable: true, IsSigner: false},
		{PublicKey: p.Pool.VaultB, IsWritable: true, IsSigner: false},
		{PublicKey: p.Pool.MintA, IsWritable: false, IsSigner: false},
		{PublicKey: p.Pool.MintB, IsWritable: false, IsSigner: false},
		{PublicKey: p.Payer, IsWritable: false, IsSigner: true},
		{PublicKey: p.TokenProgramA, IsWritable: false, IsSigner: false},
		{PublicKey: p.TokenProgramB, IsWritable: false, IsSigner: false},
		{PublicKey: referral, IsWritable: true, IsSigner: false},
		{PublicKey: eventAuth, IsWritable: false, IsSigner: false},
		{PublicKey: p.ProgramID, IsWritable: false, IsSigner: false},
	}, data), nil
}
