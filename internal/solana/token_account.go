package solana

import (
	"encoding/binary"
	"fmt"
)

// TokenAccountAmount reads SPL / Token-2022 account amount at offset 64.
func TokenAccountAmount(data []byte) (uint64, error) {
	if len(data) < 72 {
		return 0, fmt.Errorf("token account data too short: %d", len(data))
	}
	return binary.LittleEndian.Uint64(data[64:72]), nil
}
