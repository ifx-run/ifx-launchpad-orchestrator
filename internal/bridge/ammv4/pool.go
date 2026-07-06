package ammv4

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// PoolState mirrors Raydium AMM v4 LiquidityStateV4 (borsh, no anchor disc).
type PoolState struct {
	BaseVault  solana.PublicKey // pool coin token account
	QuoteVault solana.PublicKey // pool pc token account
	BaseMint   solana.PublicKey // coin mint
	QuoteMint  solana.PublicKey // pc mint
	OpenOrders solana.PublicKey
}

const (
	offBaseVault  = 336
	offQuoteVault = 368
	offBaseMint   = 400
	offQuoteMint  = 432
	offOpenOrders = 496
)

// Authority is the shared Raydium AMM v4 authority PDA.
func Authority() solana.PublicKey {
	return solana.MustPublicKeyFromBase58("5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1")
}

func DecodePoolState(data []byte) (PoolState, error) {
	if len(data) < offOpenOrders+32 {
		return PoolState{}, fmt.Errorf("amm v4 pool data too short: %d", len(data))
	}
	readPK := func(off int) solana.PublicKey {
		return solana.PublicKeyFromBytes(data[off : off+32])
	}
	return PoolState{
		BaseVault:  readPK(offBaseVault),
		QuoteVault: readPK(offQuoteVault),
		BaseMint:   readPK(offBaseMint),
		QuoteMint:  readPK(offQuoteMint),
		OpenOrders: readPK(offOpenOrders),
	}, nil
}

// Side maps swap direction to coin/pc vaults for SwapBaseInV2.
type Side struct {
	CoinVault solana.PublicKey
	PcVault   solana.PublicKey
}

func (p PoolState) Side(inputMint, outputMint solana.PublicKey) (Side, error) {
	switch {
	case inputMint.Equals(p.BaseMint) && outputMint.Equals(p.QuoteMint):
		return Side{CoinVault: p.BaseVault, PcVault: p.QuoteVault}, nil
	case inputMint.Equals(p.QuoteMint) && outputMint.Equals(p.BaseMint):
		return Side{CoinVault: p.BaseVault, PcVault: p.QuoteVault}, nil
	default:
		return Side{}, fmt.Errorf("amm v4 pool mints %s/%s do not match swap %s→%s",
			p.BaseMint, p.QuoteMint, inputMint, outputMint)
	}
}
