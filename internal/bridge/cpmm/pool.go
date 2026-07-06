package cpmm

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// PoolState mirrors Raydium CPMM pool account (CpmmPoolInfoLayout).
type PoolState struct {
	ConfigID        solana.PublicKey
	VaultA          solana.PublicKey
	VaultB          solana.PublicKey
	MintA           solana.PublicKey
	MintB           solana.PublicKey
	MintProgramA    solana.PublicKey
	MintProgramB    solana.PublicKey
	ObservationID   solana.PublicKey
}

func DecodePoolState(data []byte) (PoolState, error) {
	const minLen = 8 + 32*9 + 3 // through observationId
	if len(data) < minLen {
		return PoolState{}, fmt.Errorf("cpmm pool data too short: %d", len(data))
	}
	body := data[8:]
	readPK := func(off int) solana.PublicKey {
		return solana.PublicKeyFromBytes(body[off : off+32])
	}
	return PoolState{
		ConfigID:      readPK(0),
		VaultA:        readPK(64),
		VaultB:        readPK(96),
		MintA:         readPK(160),
		MintB:         readPK(192),
		MintProgramA:  readPK(224),
		MintProgramB:  readPK(256),
		ObservationID: readPK(288),
	}, nil
}

func Authority(programID solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("vault_and_lp_mint_auth_seed")},
		programID,
	)
	return pda, err
}

// Side maps swap direction to vaults/programs/mints.
type Side struct {
	InputMint, OutputMint     solana.PublicKey
	InputVault, OutputVault   solana.PublicKey
	InputProgram, OutputProgram solana.PublicKey
}

func (p PoolState) Side(inputMint, outputMint solana.PublicKey) (Side, error) {
	switch {
	case inputMint.Equals(p.MintA) && outputMint.Equals(p.MintB):
		return Side{
			InputMint: p.MintA, OutputMint: p.MintB,
			InputVault: p.VaultA, OutputVault: p.VaultB,
			InputProgram: p.MintProgramA, OutputProgram: p.MintProgramB,
		}, nil
	case inputMint.Equals(p.MintB) && outputMint.Equals(p.MintA):
		return Side{
			InputMint: p.MintB, OutputMint: p.MintA,
			InputVault: p.VaultB, OutputVault: p.VaultA,
			InputProgram: p.MintProgramB, OutputProgram: p.MintProgramA,
		}, nil
	default:
		return Side{}, fmt.Errorf("cpmm pool mints %s/%s do not match swap %s→%s",
			p.MintA, p.MintB, inputMint, outputMint)
	}
}
