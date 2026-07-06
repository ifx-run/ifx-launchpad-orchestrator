package cpmm

import (
	"encoding/binary"

	"github.com/gagliardetto/solana-go"
)

var discSwapBaseInput = [8]byte{143, 190, 90, 218, 196, 30, 51, 222}

type SwapParams struct {
	ProgramID       solana.PublicKey
	Payer           solana.PublicKey
	Pool            PoolState
	PoolID          solana.PublicKey
	UserInputATA    solana.PublicKey
	UserOutputATA   solana.PublicKey
	InputMint       solana.PublicKey
	OutputMint      solana.PublicKey
	AmountIn        uint64
	MinAmountOut    uint64
}

func BuildSwapBaseInput(p SwapParams) (solana.Instruction, error) {
	auth, err := Authority(p.ProgramID)
	if err != nil {
		return nil, err
	}
	side, err := p.Pool.Side(p.InputMint, p.OutputMint)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 24)
	copy(data[:8], discSwapBaseInput[:])
	binary.LittleEndian.PutUint64(data[8:16], p.AmountIn)
	binary.LittleEndian.PutUint64(data[16:24], p.MinAmountOut)
	return solana.NewInstruction(p.ProgramID, solana.AccountMetaSlice{
		{PublicKey: p.Payer, IsWritable: false, IsSigner: true},
		{PublicKey: auth, IsWritable: false, IsSigner: false},
		{PublicKey: p.Pool.ConfigID, IsWritable: false, IsSigner: false},
		{PublicKey: p.PoolID, IsWritable: true, IsSigner: false},
		{PublicKey: p.UserInputATA, IsWritable: true, IsSigner: false},
		{PublicKey: p.UserOutputATA, IsWritable: true, IsSigner: false},
		{PublicKey: side.InputVault, IsWritable: true, IsSigner: false},
		{PublicKey: side.OutputVault, IsWritable: true, IsSigner: false},
		{PublicKey: side.InputProgram, IsWritable: false, IsSigner: false},
		{PublicKey: side.OutputProgram, IsWritable: false, IsSigner: false},
		{PublicKey: side.InputMint, IsWritable: false, IsSigner: false},
		{PublicKey: side.OutputMint, IsWritable: false, IsSigner: false},
		{PublicKey: p.Pool.ObservationID, IsWritable: true, IsSigner: false},
	}, data), nil
}
