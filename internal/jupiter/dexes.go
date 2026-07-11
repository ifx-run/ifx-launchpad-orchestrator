package jupiter

import (
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/bridge"
)

func poolTypeToJupiterDex(t bridge.PoolType) ([]string, bool) {
	switch t {
	case bridge.PoolMeteoraDAMMv2:
		return []string{"Meteora DAMM v2"}, true
	case bridge.PoolRaydiumAMMv4, bridge.PoolRaydiumCPMM:
		return []string{"Raydium"}, true
	default:
		return nil, false
	}
}

// DexesForSupportedTypes derives Jupiter quote API dexes from bridge.supported_types.
// Unsupported pool types are omitted so Jupiter is not asked to route through pools we cannot build.
func DexesForSupportedTypes(supported []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range supported {
		pt, err := bridge.ParsePoolType(s)
		if err != nil {
			continue
		}
		dexes, ok := poolTypeToJupiterDex(pt)
		if !ok {
			continue
		}
		for _, d := range dexes {
			if _, dup := seen[d]; dup {
				continue
			}
			seen[d] = struct{}{}
			out = append(out, d)
		}
	}
	return out
}
