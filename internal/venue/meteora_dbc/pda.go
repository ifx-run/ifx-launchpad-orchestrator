package meteora_dbc

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
)

// CandidateAccounts returns pubkeys for Meteora DBC virtual pool detection.
// TODO: align seeds with docs/venues/meteora_dbc.md once written.
func CandidateAccounts(cfg *config.Config, baseMint solana.PublicKey) ([]solana.PublicKey, error) {
	program, err := solana.PublicKeyFromBase58(cfg.Venues.MeteoraDBC.ProgramID)
	if err != nil {
		return nil, fmt.Errorf("meteora dbc program id: %w", err)
	}

	pool, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("virtual_pool"), baseMint.Bytes()},
		program,
	)
	if err != nil {
		return nil, err
	}
	return []solana.PublicKey{pool}, nil
}
