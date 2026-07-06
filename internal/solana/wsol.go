package solana

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

// WSOLUnwrapMode selects how WSOL SPL is converted to native SOL.
type WSOLUnwrapMode int

const (
	WSOLUnwrapPartial WSOLUnwrapMode = iota
	WSOLUnwrapClose
	WSOLUnwrapLamportsAll
)

// SyncNativeInstruction marks lamports in a WSOL ATA as SPL balance (discriminator 17).
func SyncNativeInstruction(wsolATA, tokenProgram solana.PublicKey) solana.Instruction {
	return solana.NewInstruction(
		tokenProgram,
		solana.AccountMetaSlice{
			{PublicKey: wsolATA, IsWritable: true, IsSigner: false},
		},
		[]byte{17},
	)
}

// WrapSOLInstructions creates WSOL ATA (if needed), transfers lamports, SyncNative.
func WrapSOLInstructions(payer, owner, wsolMint, tokenProgram solana.PublicKey, lamports uint64) ([]solana.Instruction, error) {
	ata, err := ataAddress(owner, wsolMint, tokenProgram)
	if err != nil {
		return nil, err
	}
	create, err := createATAIdempotent(payer, owner, wsolMint, tokenProgram)
	if err != nil {
		return nil, err
	}
	transfer := system.NewTransferInstruction(lamports, payer, ata).Build()
	sync := SyncNativeInstruction(ata, tokenProgram)
	return []solana.Instruction{create, transfer, sync}, nil
}

// UnwrapWSOLInstructions unwraps WSOL SPL to native SOL for owner.
// Partial: SyncNative + UnwrapLamports(amount). Full close: SyncNative + CloseAccount. Full keep ATA: SyncNative + UnwrapLamports(all).
func UnwrapWSOLInstructions(owner, wsolMint, tokenProgram solana.PublicKey, lamports uint64, mode WSOLUnwrapMode) ([]solana.Instruction, error) {
	if lamports == 0 && mode == WSOLUnwrapPartial {
		return nil, fmt.Errorf("unwrap amount must be positive")
	}
	srcATA, err := ataAddress(owner, wsolMint, tokenProgram)
	if err != nil {
		return nil, err
	}
	sync := SyncNativeInstruction(srcATA, tokenProgram)
	switch mode {
	case WSOLUnwrapClose:
		closeIx, err := CloseWSOLATA(owner, wsolMint, tokenProgram)
		if err != nil {
			return nil, err
		}
		return []solana.Instruction{sync, closeIx}, nil
	case WSOLUnwrapLamportsAll:
		unwrap := UnwrapLamportsInstruction(srcATA, owner, owner, tokenProgram, nil)
		return []solana.Instruction{sync, unwrap}, nil
	default:
		amount := lamports
		unwrap := UnwrapLamportsInstruction(srcATA, owner, owner, tokenProgram, &amount)
		return []solana.Instruction{sync, unwrap}, nil
	}
}

// CloseWSOLATA unwraps full WSOL balance to native lamports on owner by closing the ATA.
func CloseWSOLATA(owner, wsolMint, tokenProgram solana.PublicKey) (solana.Instruction, error) {
	ata, err := ataAddress(owner, wsolMint, tokenProgram)
	if err != nil {
		return nil, err
	}
	return solana.NewInstruction(
		tokenProgram,
		solana.AccountMetaSlice{
			{PublicKey: ata, IsWritable: true, IsSigner: false},
			{PublicKey: owner, IsWritable: true, IsSigner: false},
			{PublicKey: owner, IsWritable: false, IsSigner: true},
		},
		[]byte{9},
	), nil
}

func ataAddress(owner, mint, tokenProgram solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{owner.Bytes(), tokenProgram.Bytes(), mint.Bytes()},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	return pda, err
}

func createATAIdempotent(payer, owner, mint, tokenProgram solana.PublicKey) (solana.Instruction, error) {
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
