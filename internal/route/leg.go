package route

// PairClass classifies a mint pair for routing.
type PairClass int

const (
	PairQuoteSwap PairClass = iota
	PairBuyLaunchpad
	PairSellLaunchpad
	PairSwapLaunchpad
	PairSOLSettlement
)

func (c PairClass) String() string {
	switch c {
	case PairQuoteSwap:
		return "quote_swap"
	case PairBuyLaunchpad:
		return "buy_launchpad"
	case PairSellLaunchpad:
		return "sell_launchpad"
	case PairSwapLaunchpad:
		return "swap_launchpad"
	case PairSOLSettlement:
		return "sol_settlement"
	default:
		return "unknown"
	}
}

// ClassifyPair decides routing from input/output mints and quote set.
func ClassifyPair(inputMint, outputMint string, isQuote func(string) bool) PairClass {
	inQ := isQuote(inputMint)
	outQ := isQuote(outputMint)
	switch {
	case inQ && outQ:
		return PairQuoteSwap
	case inQ && !outQ:
		return PairBuyLaunchpad
	case !inQ && outQ:
		return PairSellLaunchpad
	default:
		return PairSwapLaunchpad
	}
}

// LegKind is one hop in a route.
type LegKind string

const (
	LegLaunchpad      LegKind = "launchpad"
	LegQuoteBridge    LegKind = "quote_bridge"
	LegSOLSettlement  LegKind = "sol_settlement"
)

const (
	SettlementNativeSOL = "native_sol"
	SettlementWSOLSPL   = "wsol_spl"
)

// IsSOLSettlementConvert is true for WSOL mint with native_sol ↔ wsol_spl settlement flip.
func IsSOLSettlementConvert(inputMint, outputMint, inputSettlement, outputSettlement, wsolMint string) bool {
	if inputMint != wsolMint || outputMint != wsolMint {
		return false
	}
	return (inputSettlement == SettlementWSOLSPL && outputSettlement == SettlementNativeSOL) ||
		(inputSettlement == SettlementNativeSOL && outputSettlement == SettlementWSOLSPL)
}

// SOLSettlementUnwrap reports WSOL SPL → native SOL.
func SOLSettlementUnwrap(inputSettlement, outputSettlement string) bool {
	return inputSettlement == SettlementWSOLSPL && outputSettlement == SettlementNativeSOL
}

type Leg struct {
	Kind       LegKind `json:"kind"`
	InputMint  string  `json:"inputMint"`
	OutputMint string  `json:"outputMint"`
	Venue      string  `json:"venue,omitempty"`
	PoolID     string  `json:"poolId,omitempty"`
	PoolType   string  `json:"poolType,omitempty"`
}
