package jupiter_test

import (
	"testing"

	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/jupiter"
)

func TestDexesForSupportedTypes(t *testing.T) {
	cases := []struct {
		name  string
		types []string
		want  []string
	}{
		{"meteora only", []string{"meteora_damm_v2"}, []string{"Meteora DAMM v2"}},
		{"raydium v4", []string{"raydium_amm_v4"}, []string{"Raydium"}},
		{"raydium both deduped", []string{"raydium_amm_v4", "raydium_cpmm"}, []string{"Raydium"}},
		{"meteora and raydium", []string{"meteora_damm_v2", "raydium_cpmm"}, []string{"Meteora DAMM v2", "Raydium"}},
		{"unknown ignored", []string{"orca_whirlpool", "meteora_damm_v2"}, []string{"Meteora DAMM v2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := jupiter.DexesForSupportedTypes(tc.types)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}
