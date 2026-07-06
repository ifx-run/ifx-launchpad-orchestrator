package orchestrator

import (
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

// JitoTipInstruction returns a SystemProgram transfer to the configured Jito tip account.
func JitoTipInstruction(cfg *config.Config, feePayer solana.PublicKey, lamports uint64) (solana.Instruction, error) {
	tipAcct, err := solana.PublicKeyFromBase58(cfg.Jito.TipAccount)
	if err != nil {
		return nil, fmt.Errorf("jito tip_account: %w", err)
	}
	return system.NewTransferInstruction(lamports, feePayer, tipAcct).Build(), nil
}

func jitoTipLamports(cfg *config.Config, mev bool) uint64 {
	if !mev || !cfg.Jito.Enabled {
		return 0
	}
	return cfg.Jito.TipLamports
}
