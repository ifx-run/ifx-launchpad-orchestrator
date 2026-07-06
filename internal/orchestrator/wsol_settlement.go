package orchestrator

import (
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/gagliardetto/solana-go"
)

const (
	SettlementNativeSOL = "native_sol"
	SettlementWSOLSPL   = "wsol_spl"
	SettlementSPL       = "spl"
)

// NormalizeSettlement fills defaults: WSOL mint defaults to native_sol; other quotes to spl.
func NormalizeSettlement(mint, settlement, wsolMint string) string {
	if mint != wsolMint {
		if settlement == "" {
			return SettlementSPL
		}
		return settlement
	}
	switch settlement {
	case SettlementWSOLSPL, SettlementNativeSOL:
		return settlement
	default:
		return SettlementNativeSOL
	}
}

func wantsNativeSOL(settlement string) bool {
	return settlement == "" || settlement == SettlementNativeSOL
}

// settlementNoPriorityTier is used for SOL/WSOL convert txs (no compute-budget ixs).
func settlementNoPriorityTier() config.PriorityFeeTier {
	return config.PriorityFeeTier{}
}

// Settlement unwrap mode keys in quote.builds (full-balance only).
const (
	SettlementModeClose     = "close"
	SettlementModeUnwrapAll = "unwrapAll"
)

func settlementBuildKey(variant, mode string) string {
	if mode == "" {
		return variant
	}
	return variant + "_" + mode
}

func (s *Service) quoteSettlementRepayDeduction(ataCreates int) uint64 {
	return EstimateRepayLamports(s.cfg, settlementNoPriorityTier(), 2, ataCreates, 0)
}

func (s *Service) appendUnwrapWSOLIfNeeded(
	ixs *[]solana.Instruction,
	outputSettlement string,
	outputMint, user, wsolMint solana.PublicKey,
) error {
	if !outputMint.Equals(wsolMint) || !wantsNativeSOL(outputSettlement) {
		return nil
	}
	closeIx, err := solpkg.CloseWSOLATA(user, wsolMint, solana.TokenProgramID)
	if err != nil {
		return err
	}
	*ixs = append(*ixs, closeIx)
	return nil
}
