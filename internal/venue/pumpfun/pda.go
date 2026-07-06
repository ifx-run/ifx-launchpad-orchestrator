package pumpfun

import (
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/gagliardetto/solana-go"
)

const globalSeed = "global"

// GlobalPDA returns the pump global config account.
func GlobalPDA(cfg *config.Config) (solana.PublicKey, error) {
	if cfg.Venues.Pump.Global != "" {
		return solana.PublicKeyFromBase58(cfg.Venues.Pump.Global)
	}
	program, err := solana.PublicKeyFromBase58(cfg.Venues.Pump.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	global, _, err := solana.FindProgramAddress([][]byte{[]byte(globalSeed)}, program)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return global, nil
}

// CandidateAccounts returns pubkeys to fetch for pump.fun bonding curve detection.
func CandidateAccounts(cfg *config.Config, baseMint solana.PublicKey) ([]solana.PublicKey, error) {
	program, err := solana.PublicKeyFromBase58(cfg.Venues.Pump.ProgramID)
	if err != nil {
		return nil, fmt.Errorf("pump program id: %w", err)
	}

	bondingCurve, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("bonding-curve"), baseMint.Bytes()},
		program,
	)
	if err != nil {
		return nil, fmt.Errorf("pump bonding curve PDA: %w", err)
	}

	global, err := GlobalPDA(cfg)
	if err != nil {
		return nil, err
	}

	return []solana.PublicKey{bondingCurve, global}, nil
}

func BondingCurvePDAFromMint(baseMint solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("bonding-curve"), baseMint.Bytes()},
		ProgramID(),
	)
	return pda, err
}
