package bridge

import (
	"fmt"
	"strings"
)

type PoolType string

const (
	PoolRaydiumAMMv4   PoolType = "raydium_amm_v4"
	PoolRaydiumCPMM    PoolType = "raydium_cpmm"
	PoolMeteoraDAMMv2  PoolType = "meteora_damm_v2"
)

type PoolMeta struct {
	Type            PoolType
	ProgramID       string
	SwapIxAccounts  int
}

var poolRegistry = map[PoolType]PoolMeta{
	PoolRaydiumAMMv4:  {Type: PoolRaydiumAMMv4, SwapIxAccounts: 8},
	PoolRaydiumCPMM:   {Type: PoolRaydiumCPMM, SwapIxAccounts: 13},
	PoolMeteoraDAMMv2: {Type: PoolMeteoraDAMMv2, SwapIxAccounts: 14},
}

func ParsePoolType(s string) (PoolType, error) {
	t := PoolType(strings.TrimSpace(s))
	if _, ok := poolRegistry[t]; !ok {
		return "", fmt.Errorf("unknown pool type %q", s)
	}
	return t, nil
}

func (t PoolType) SwapIxAccounts() int {
	if m, ok := poolRegistry[t]; ok {
		return m.SwapIxAccounts
	}
	return 0
}

func IsSupported(poolType PoolType, supported []string, maxAccounts int) bool {
	for _, s := range supported {
		if PoolType(s) == poolType {
			return poolRegistry[poolType].SwapIxAccounts <= maxAccounts
		}
	}
	return false
}

type DiscoveredPool struct {
	PoolID      string
	PoolType    PoolType
	InputMint   string
	OutputMint  string
	InAmount    string
	OutAmount   string
	PriceImpact string
	Label       string
}
