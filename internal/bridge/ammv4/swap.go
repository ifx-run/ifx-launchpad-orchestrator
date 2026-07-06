package ammv4

import (
	"encoding/binary"

	"github.com/gagliardetto/solana-go"
)

const discSwapBaseInV2 uint8 = 16

type SwapParams struct {
	ProgramID     solana.PublicKey
	Payer         solana.PublicKey
	Pool          PoolState
	PoolID        solana.PublicKey
	UserInputATA  solana.PublicKey
	UserOutputATA solana.PublicKey
	InputMint     solana.PublicKey
	OutputMint    solana.PublicKey
	TokenProgram  solana.PublicKey
	AmountIn      uint64
	MinAmountOut  uint64
}

// BuildSwapBaseInV2 builds Raydium AMM v4 SwapBaseInV2 (8 accounts; on-chain processor skips OpenBook).
func BuildSwapBaseInV2(p SwapParams) (solana.Instruction, error) {
	if _, err := p.Pool.Side(p.InputMint, p.OutputMint); err != nil {
		return nil, err
	}

	data := make([]byte, 17)
	data[0] = discSwapBaseInV2
	binary.LittleEndian.PutUint64(data[1:9], p.AmountIn)
	binary.LittleEndian.PutUint64(data[9:17], p.MinAmountOut)

	tokenProgram := p.TokenProgram
	if tokenProgram.IsZero() {
		tokenProgram = solana.TokenProgramID
	}

	return solana.NewInstruction(p.ProgramID, solana.AccountMetaSlice{
		{PublicKey: tokenProgram, IsWritable: false, IsSigner: false},
		{PublicKey: p.PoolID, IsWritable: true, IsSigner: false},
		{PublicKey: Authority(), IsWritable: false, IsSigner: false},
		{PublicKey: p.Pool.BaseVault, IsWritable: true, IsSigner: false},
		{PublicKey: p.Pool.QuoteVault, IsWritable: true, IsSigner: false},
		{PublicKey: p.UserInputATA, IsWritable: true, IsSigner: false},
		{PublicKey: p.UserOutputATA, IsWritable: true, IsSigner: false},
		{PublicKey: p.Payer, IsWritable: false, IsSigner: true},
	}, data), nil
}
