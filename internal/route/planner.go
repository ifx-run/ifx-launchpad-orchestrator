package route

// PlannedRoute is an ordered leg chain for one user intent.
type PlannedRoute struct {
	Legs     []Leg `json:"legs"`
	HopCount int   `json:"hopCount"`
}

// PlanLaunchpadRoute expands buy/sell/swap into 1–3 legs using pool-native quote Q_native.
// qNative is the launchpad pool's quote mint (from on-chain state).
func PlanLaunchpadRoute(
	pairClass PairClass,
	inputMint, outputMint, qNative string,
	isQuote func(string) bool,
) PlannedRoute {
	switch pairClass {
	case PairBuyLaunchpad:
		return planBuy(inputMint, outputMint, qNative, isQuote)
	case PairSellLaunchpad:
		return planSell(inputMint, outputMint, qNative, isQuote)
	case PairSwapLaunchpad:
		return planSwapAB(inputMint, outputMint, qNative, isQuote)
	default:
		return PlannedRoute{}
	}
}

func planBuy(qPay, baseMint, qNative string, isQuote func(string) bool) PlannedRoute {
	if isQuote(qPay) && qPay == qNative {
		return singleLeg(LegLaunchpad, qPay, baseMint)
	}
	// Q_pay → Q_native → base
	return PlannedRoute{
		Legs: []Leg{
			{Kind: LegQuoteBridge, InputMint: qPay, OutputMint: qNative},
			{Kind: LegLaunchpad, InputMint: qNative, OutputMint: baseMint},
		},
		HopCount: 2,
	}
}

func planSell(baseMint, qRecv, qNative string, isQuote func(string) bool) PlannedRoute {
	if isQuote(qRecv) && qRecv == qNative {
		return singleLeg(LegLaunchpad, baseMint, qRecv)
	}
	return PlannedRoute{
		Legs: []Leg{
			{Kind: LegLaunchpad, InputMint: baseMint, OutputMint: qNative},
			{Kind: LegQuoteBridge, InputMint: qNative, OutputMint: qRecv},
		},
		HopCount: 2,
	}
}

func planSwapAB(baseA, baseB, qNativeA string, isQuote func(string) bool) PlannedRoute {
	// v1 caller supplies qNative for A; B's native quote resolved at detect time for 3-hop.
	_ = isQuote
	// Same Q_native: A → Q → B (2 launchpad legs)
	return PlannedRoute{
		Legs: []Leg{
			{Kind: LegLaunchpad, InputMint: baseA, OutputMint: qNativeA},
			{Kind: LegLaunchpad, InputMint: qNativeA, OutputMint: baseB},
		},
		HopCount: 2,
	}
}

func singleLeg(kind LegKind, in, out string) PlannedRoute {
	return PlannedRoute{
		Legs:     []Leg{{Kind: kind, InputMint: in, OutputMint: out}},
		HopCount: 1,
	}
}

// HasBridgeLeg reports whether the planned route includes a quote bridge hop.
func HasBridgeLeg(p PlannedRoute) bool {
	for _, leg := range p.Legs {
		if leg.Kind == LegQuoteBridge {
			return true
		}
	}
	return false
}

// RouteInvolvesSOL reports whether any leg touches WSOL / native SOL routing.
func RouteInvolvesSOL(legs []Leg, wsolMint string) bool {
	for _, leg := range legs {
		if leg.InputMint == wsolMint || leg.OutputMint == wsolMint {
			return true
		}
	}
	return false
}

// RouteInputIsNativeSOL reports whether the user pays the route with native SOL lamports.
// WSOL SPL input (wsol_spl) is allowed — repay can come from unwrap output or swap proceeds.
func RouteInputIsNativeSOL(inputSettlement string) bool {
	return inputSettlement == "native_sol"
}

// RouteHasSOLStream reports whether mints, settlements, or legs touch SOL/WSOL.
func RouteHasSOLStream(inputMint, outputMint, outputSettlement, wsolMint string, legs []Leg) bool {
	if inputMint == wsolMint || outputMint == wsolMint {
		return true
	}
	if outputSettlement == "native_sol" || outputSettlement == "wsol_spl" {
		return true
	}
	return RouteInvolvesSOL(legs, wsolMint)
}

// SponsoredRepayEligible is true when sponsor can be repaid from a SOL/WSOL stream on the route.
// Excluded: (1) native SOL pays the route; (2) no SOL/WSOL on input, output, or legs.
func SponsoredRepayEligible(inputMint, outputMint, inputSettlement, outputSettlement, wsolMint string, legs []Leg) bool {
	if len(legs) == 1 && legs[0].Kind == LegQuoteBridge {
		return QuoteSwapSponsoredEligible(inputMint, outputMint, inputSettlement, outputSettlement, wsolMint)
	}
	if RouteInputIsNativeSOL(inputSettlement) {
		return false
	}
	return RouteHasSOLStream(inputMint, outputMint, outputSettlement, wsolMint, legs)
}

// QuoteSwapSponsoredEligible is true when repay can come from WSOL swap output (unwrap to SOL for repay).
// Paying with native SOL does not qualify — user already holds gas funds.
func QuoteSwapSponsoredEligible(inputMint, outputMint, inputSettlement, outputSettlement, wsolMint string) bool {
	if inputSettlement == "native_sol" {
		return false
	}
	if outputMint != wsolMint {
		return false
	}
	switch outputSettlement {
	case "native_sol", "wsol_spl", "":
		return true
	default:
		return false
	}
}
