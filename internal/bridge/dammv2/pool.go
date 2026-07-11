package dammv2

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// PoolState mirrors Meteora DAMM v2 (cp-amm) Pool account fields needed for swap2.
// Layout: 8-byte discriminator + PoolFeesStruct (160) + mints/vaults.
type PoolState struct {
	MintA   solana.PublicKey
	MintB   solana.PublicKey
	VaultA  solana.PublicKey
	VaultB  solana.PublicKey
}

const poolFeesSize = 160

func DecodePoolState(data []byte) (PoolState, error) {
	const minLen = 8 + poolFeesSize + 32*4
	if len(data) < minLen {
		return PoolState{}, fmt.Errorf("damm v2 pool data too short: %d", len(data))
	}
	body := data[8:]
	readPK := func(off int) solana.PublicKey {
		return solana.PublicKeyFromBytes(body[off : off+32])
	}
	return PoolState{
		MintA:  readPK(poolFeesSize),
		MintB:  readPK(poolFeesSize + 32),
		VaultA: readPK(poolFeesSize + 64),
		VaultB: readPK(poolFeesSize + 96),
	}, nil
}

type Side struct {
	InputMint, OutputMint       solana.PublicKey
	InputVault, OutputVault     solana.PublicKey
	InputProgram, OutputProgram solana.PublicKey
}

func (p PoolState) Side(inputMint, outputMint, programA, programB solana.PublicKey) (Side, error) {
	switch {
	case inputMint.Equals(p.MintA) && outputMint.Equals(p.MintB):
		return Side{
			InputMint: p.MintA, OutputMint: p.MintB,
			InputVault: p.VaultA, OutputVault: p.VaultB,
			InputProgram: programA, OutputProgram: programB,
		}, nil
	case inputMint.Equals(p.MintB) && outputMint.Equals(p.MintA):
		return Side{
			InputMint: p.MintB, OutputMint: p.MintA,
			InputVault: p.VaultB, OutputVault: p.VaultA,
			InputProgram: programB, OutputProgram: programA,
		}, nil
	default:
		return Side{}, fmt.Errorf("damm v2 pool mints %s/%s do not match swap %s→%s",
			p.MintA, p.MintB, inputMint, outputMint)
	}
}
