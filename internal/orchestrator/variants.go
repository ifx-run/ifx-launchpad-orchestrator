package orchestrator

import (
	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
)

// Build variant keys returned in quote.builds / capabilities.
const (
	VariantSelfFunded       = "selfFunded"
	VariantSelfFundedMev    = "selfFundedMev"
	VariantSponsoredSwap    = "sponsoredSwap"
	VariantSponsoredSwapMev = "sponsoredSwapMev"
)

// VariantMode selects sponsored swap and/or MEV (Jito tip).
type VariantMode struct {
	Sponsored bool
	Mev       bool
}

func (m VariantMode) Key() string {
	switch {
	case m.Sponsored && m.Mev:
		return VariantSponsoredSwapMev
	case m.Sponsored:
		return VariantSponsoredSwap
	case m.Mev:
		return VariantSelfFundedMev
	default:
		return VariantSelfFunded
	}
}

// AllVariantModes returns the four modes in stable order.
func AllVariantModes() []VariantMode {
	return []VariantMode{
		{},
		{Mev: true},
		{Sponsored: true},
		{Sponsored: true, Mev: true},
	}
}

type Capability struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
}

type VariantsResult struct {
	Builds                map[string]*BuildResult `json:"-"`
	Capabilities          map[string]Capability   `json:"-"`
	DefaultVariant        string                  `json:"-"`
	SettlementFullBalance bool                    `json:"-"`
	SettlementModes       []string                `json:"-"`
}

func newVariantsResult() *VariantsResult {
	return &VariantsResult{
		Builds:         make(map[string]*BuildResult),
		Capabilities:   make(map[string]Capability),
		DefaultVariant: VariantSelfFunded,
	}
}

// TxPlan is the IO-free instruction bundle before variant-specific tip / compile.
type TxPlan struct {
	User               solana.PublicKey
	Tier               config.PriorityFeeTier
	Instructions       []solana.Instruction
	ServiceFeeLamports uint64
	ATACreates         int
	SponsoredEligible  bool
}
