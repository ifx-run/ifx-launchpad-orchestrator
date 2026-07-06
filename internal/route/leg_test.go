package route_test

import (
	"testing"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
)

func TestClassifyPair(t *testing.T) {
	isQuote := func(m string) bool {
		return m == "SOL" || m == "USDC"
	}

	cases := []struct {
		in, out string
		want    route.PairClass
	}{
		{"SOL", "USDC", route.PairQuoteSwap},
		{"USDC", "SOL", route.PairQuoteSwap},
		{"USDC", "TOKEN", route.PairBuyLaunchpad},
		{"TOKEN", "USDC", route.PairSellLaunchpad},
		{"TOKENA", "TOKENB", route.PairSwapLaunchpad},
	}

	for _, tc := range cases {
		got := route.ClassifyPair(tc.in, tc.out, isQuote)
		if got != tc.want {
			t.Fatalf("ClassifyPair(%q,%q)=%v want %v", tc.in, tc.out, got, tc.want)
		}
	}
}
