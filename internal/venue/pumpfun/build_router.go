package pumpfun

import (
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/gagliardetto/solana-go"
)

// QuoteKind classifies pool quote settlement for build/quote paths.
type QuoteKind int

const (
	QuoteNativeSOL QuoteKind = iota
	QuoteSPL
)

func QuoteKindFor(cfg *config.Config, curve BondingCurve) QuoteKind {
	wsol, _ := solana.PublicKeyFromBase58(cfg.Quotes.WSOLMint)
	if IsNativeSolQuotePair(curve.QuoteMint, wsol) {
		return QuoteNativeSOL
	}
	return QuoteSPL
}

// BuildBuy selects SOL or SPL-quote buy instruction builder.
func BuildBuy(p BuildParams, kind QuoteKind) ([]solana.Instruction, error) {
	switch kind {
	case QuoteNativeSOL:
		return BuildBuyInstructions(p)
	case QuoteSPL:
		return BuildBuyV2Instructions(p)
	default:
		return nil, fmt.Errorf("unsupported quote kind")
	}
}

// BuildSell selects SOL or SPL-quote sell instruction builder.
func BuildSell(p BuildParams, kind QuoteKind) ([]solana.Instruction, error) {
	switch kind {
	case QuoteNativeSOL:
		return BuildSellCoreInstructions(p)
	case QuoteSPL:
		return BuildSellV2Instructions(p)
	default:
		return nil, fmt.Errorf("unsupported quote kind")
	}
}
