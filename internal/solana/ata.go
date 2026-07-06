package solana

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
)

const Token2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"

// ATAPair holds legacy SPL Token and Token-2022 associated token addresses.
type ATAPair struct {
	Legacy    solana.PublicKey
	Token2022 solana.PublicKey
}

func DeriveATAPair(owner, mint solana.PublicKey) (ATAPair, error) {
	legacy, _, err := solana.FindAssociatedTokenAddress(owner, mint)
	if err != nil {
		return ATAPair{}, fmt.Errorf("derive legacy ATA: %w", err)
	}

	token2022Program, err := solana.PublicKeyFromBase58(Token2022ProgramID)
	if err != nil {
		return ATAPair{}, err
	}
	token2022, _, err := solana.FindProgramAddress(
		[][]byte{owner.Bytes(), token2022Program.Bytes(), mint.Bytes()},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return ATAPair{}, fmt.Errorf("derive token2022 ATA: %w", err)
	}

	return ATAPair{Legacy: legacy, Token2022: token2022}, nil
}

// SelectATA picks legacy or Token-2022 ATA based on mint owner program.
func SelectATA(pair ATAPair, mintOwner solana.PublicKey) solana.PublicKey {
	token2022, _ := solana.PublicKeyFromBase58(Token2022ProgramID)
	if mintOwner.Equals(token2022) {
		return pair.Token2022
	}
	return pair.Legacy
}
