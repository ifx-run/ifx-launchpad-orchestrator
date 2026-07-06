package raydium_launchpad

import (
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/gagliardetto/solana-go"
)

// CandidateAccounts returns pubkeys for Raydium Launchpad pool detection.
// TODO: align seeds with docs/venues/raydium_launchpad.md once written.
func CandidateAccounts(cfg *config.Config, baseMint solana.PublicKey) ([]solana.PublicKey, error) {
	program, err := solana.PublicKeyFromBase58(cfg.Venues.RaydiumLaunchpad.ProgramID)
	if err != nil {
		return nil, fmt.Errorf("raydium launchpad program id: %w", err)
	}

	// Placeholder seed layout — replace when venue doc is finalized.
	pool, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("launchpad_pool"), baseMint.Bytes()},
		program,
	)
	if err != nil {
		return nil, err
	}
	return []solana.PublicKey{pool}, nil
}
