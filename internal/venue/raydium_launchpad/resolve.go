package raydium_launchpad

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue"
)

// Resolve detects Raydium LaunchLab pool for base mint. Scaffold — full decode pending.
func Resolve(cfg *config.Config, accounts map[solana.PublicKey]*rpc.Account, baseMint solana.PublicKey) (*venue.Detection, error) {
	_ = cfg
	_ = accounts
	_ = baseMint
	return nil, fmt.Errorf("raydium_launchpad: venue build not implemented")
}
