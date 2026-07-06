package jupiter

import (
	"strings"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/bridge"
)

// LabelToPoolType maps Jupiter routePlan swapInfo.label to internal pool type.
func LabelToPoolType(label string) (bridge.PoolType, bool) {
	n := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(n, "cpmm"):
		return bridge.PoolRaydiumCPMM, true
	case n == "raydium" || strings.Contains(n, "raydium v4") || strings.Contains(n, "raydium_v4"):
		return bridge.PoolRaydiumAMMv4, true
	default:
		return "", false
	}
}
