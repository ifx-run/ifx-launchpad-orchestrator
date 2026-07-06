package util

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// ParseUIAmount converts a decimal UI string to raw integer units.
func ParseUIAmount(amount string, decimals uint8) (uint64, error) {
	s := strings.TrimSpace(amount)
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}
	parts := strings.SplitN(s, ".", 2)
	whole := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(frac) > int(decimals) {
		return 0, fmt.Errorf("too many decimal places")
	}
	for len(frac) < int(decimals) {
		frac += "0"
	}
	combined := whole + frac
	combined = strings.TrimLeft(combined, "0")
	if combined == "" {
		return 0, nil
	}
	v := new(big.Int)
	if _, ok := v.SetString(combined, 10); !ok {
		return 0, fmt.Errorf("invalid amount %q", amount)
	}
	if !v.IsUint64() {
		return 0, fmt.Errorf("amount overflow")
	}
	return v.Uint64(), nil
}

// MinOut applies slippage bps to expected output (floor).
func MinOut(expected uint64, slippageBPS int) uint64 {
	if slippageBPS <= 0 {
		return expected
	}
	num := new(big.Int).SetUint64(expected)
	num.Mul(num, big.NewInt(int64(10_000-slippageBPS)))
	num.Div(num, big.NewInt(10_000))
	return num.Uint64()
}

// ApplyBPS returns amount * bps / 10000 (floor).
func ApplyBPS(amount uint64, bps uint16) uint64 {
	if amount == 0 || bps == 0 {
		return 0
	}
	num := new(big.Int).SetUint64(amount)
	num.Mul(num, big.NewInt(int64(bps)))
	num.Div(num, big.NewInt(10_000))
	return num.Uint64()
}

// ResolveInputAmount prefers exact on-chain raw units when inputAmountRaw is set.
// Exact-in builds must use raw for MAX / full-balance sells to avoid UI float rounding.
func ResolveInputAmount(uiAmount, rawAmount string, decimals uint8) (uint64, error) {
	if s := strings.TrimSpace(rawAmount); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid inputAmountRaw %q: %w", rawAmount, err)
		}
		return v, nil
	}
	return ParseUIAmount(uiAmount, decimals)
}
