package jupiter_test

import (
	"testing"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/bridge"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/jupiter"
)

func TestLabelToPoolType(t *testing.T) {
	cases := []struct {
		label string
		want  bridge.PoolType
		ok    bool
	}{
		{"Raydium CPMM", bridge.PoolRaydiumCPMM, true},
		{"Raydium", bridge.PoolRaydiumAMMv4, true},
		{"Meteora DLMM", "", false},
	}

	for _, tc := range cases {
		got, ok := jupiter.LabelToPoolType(tc.label)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("LabelToPoolType(%q)=%q,%v want %q,%v", tc.label, got, ok, tc.want, tc.ok)
		}
	}
}
