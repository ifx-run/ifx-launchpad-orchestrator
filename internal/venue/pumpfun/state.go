package pumpfun

import (
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

const (
	anchorAccountDiscriminator = 8
	defaultPubkeyBase58        = "11111111111111111111111111111111"
)

// BondingCurve mirrors on-chain pump bonding curve state (v2 tail optional).
type BondingCurve struct {
	VirtualTokenReserves uint64
	VirtualSolReserves   uint64
	RealTokenReserves    uint64
	RealSolReserves      uint64
	TokenTotalSupply     uint64
	Complete             bool
	Creator              solana.PublicKey
	IsMayhemMode         bool
	IsCashbackCoin       bool
	QuoteMint            solana.PublicKey
}

// Global holds pump program global config fields needed for quote math.
type Global struct {
	FeeBasisPoints        uint64
	CreatorFeeBasisPoints uint64
}

func DecodeBondingCurve(data []byte) (BondingCurve, error) {
	if len(data) < anchorAccountDiscriminator+73 {
		return BondingCurve{}, fmt.Errorf("bonding curve data too short: %d", len(data))
	}
	body := data[anchorAccountDiscriminator:]
	curve := BondingCurve{
		VirtualTokenReserves: binary.LittleEndian.Uint64(body[0:8]),
		VirtualSolReserves:   binary.LittleEndian.Uint64(body[8:16]),
		RealTokenReserves:    binary.LittleEndian.Uint64(body[16:24]),
		RealSolReserves:      binary.LittleEndian.Uint64(body[24:32]),
		TokenTotalSupply:     binary.LittleEndian.Uint64(body[32:40]),
		Complete:             body[40] != 0,
		Creator:              solana.PublicKeyFromBytes(body[41:73]),
	}
	if len(body) > 73 {
		curve.IsMayhemMode = body[73] != 0
	}
	if len(body) > 74 {
		curve.IsCashbackCoin = body[74] != 0
	}
	if len(body) >= 107 {
		curve.QuoteMint = solana.PublicKeyFromBytes(body[75:107])
	}
	return curve, nil
}

// CashbackEnabled reads is_cashback_coin from decoded bonding curve account data.
func CashbackEnabled(data []byte) bool {
	if len(data) < anchorAccountDiscriminator+75 {
		return false
	}
	body := data[anchorAccountDiscriminator:]
	return body[74] != 0
}

func DecodeGlobal(data []byte) (Global, error) {
	if len(data) < anchorAccountDiscriminator+154 {
		return Global{}, fmt.Errorf("global data too short: %d", len(data))
	}
	body := data[anchorAccountDiscriminator:]
	return Global{
		FeeBasisPoints:        binary.LittleEndian.Uint64(body[97:105]),
		CreatorFeeBasisPoints: binary.LittleEndian.Uint64(body[146:154]),
	}, nil
}

func IsLegacyQuoteMint(mint solana.PublicKey) bool {
	def, _ := solana.PublicKeyFromBase58(defaultPubkeyBase58)
	return mint.Equals(def)
}

// IsNativeSolQuotePair reports whether the bonding curve is SOL-paired (native lamports path).
// On-chain sentinel: Pubkey::default() (legacy), zero (pre-v2 layout), or wrapped SOL mint.
func IsNativeSolQuotePair(onChain, wsolMint solana.PublicKey) bool {
	if onChain.IsZero() || IsLegacyQuoteMint(onChain) {
		return true
	}
	return onChain.Equals(wsolMint)
}

func MintDecimals(data []byte) (uint8, error) {
	if len(data) < 45 {
		return 0, fmt.Errorf("mint data too short")
	}
	return data[44], nil
}

func DefaultPubkey() solana.PublicKey {
	pk, _ := solana.PublicKeyFromBase58(defaultPubkeyBase58)
	return pk
}
