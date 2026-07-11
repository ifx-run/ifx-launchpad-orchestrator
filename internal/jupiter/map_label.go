package jupiter

import (
	"strings"

	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/bridge"
)

// LabelToPoolType maps Jupiter routePlan swapInfo.label to internal pool type.
func LabelToPoolType(label string) (bridge.PoolType, bool) {
	n := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(n, "cpmm") && strings.Contains(n, "raydium"):
		return bridge.PoolRaydiumCPMM, true
	case n == "raydium" || strings.Contains(n, "raydium v4") || strings.Contains(n, "raydium_v4"):
		return bridge.PoolRaydiumAMMv4, true
	case strings.Contains(n, "damm") && strings.Contains(n, "meteora"):
		return bridge.PoolMeteoraDAMMv2, true
	case strings.Contains(n, "meteora") && strings.Contains(n, "cp"):
		return bridge.PoolMeteoraDAMMv2, true
	default:
		return "", false
	}
}
