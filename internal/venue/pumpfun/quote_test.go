package pumpfun

import (
	"encoding/binary"
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestBuyBaseOut_basic(t *testing.T) {
	global := Global{FeeBasisPoints: 100, CreatorFeeBasisPoints: 0}
	curve := BondingCurve{
		VirtualTokenReserves: 1_073_000_000_000_000,
		VirtualSolReserves:   30_000_000_000,
		RealTokenReserves:    793_100_000_000_000,
		TokenTotalSupply:     1_000_000_000_000_000,
		Creator:              DefaultPubkey(),
	}
	out := BuyBaseOut(global, curve, 100_000_000) // 0.1 SOL
	if out == 0 {
		t.Fatal("expected non-zero base out")
	}
}

func TestSellQuoteOut_basic(t *testing.T) {
	global := Global{FeeBasisPoints: 100, CreatorFeeBasisPoints: 0}
	curve := BondingCurve{
		VirtualTokenReserves: 1_073_000_000_000_000,
		VirtualSolReserves:   30_000_000_000,
		RealTokenReserves:    793_100_000_000_000,
		TokenTotalSupply:     1_000_000_000_000_000,
		Creator:              DefaultPubkey(),
	}
	out := SellQuoteOut(global, curve, 1_000_000_000_000)
	if out == 0 {
		t.Fatal("expected non-zero quote out")
	}
}

func TestDecodeBondingCurve_quoteMintOffset(t *testing.T) {
	def := DefaultPubkey()
	buf := make([]byte, 8+107)
	copy(buf[:8], []byte{1, 2, 3, 4, 5, 6, 7, 8}) // discriminator padding
	body := buf[8:]
	binary.LittleEndian.PutUint64(body[0:8], 1)
	binary.LittleEndian.PutUint64(body[8:16], 2)
	binary.LittleEndian.PutUint64(body[16:24], 3)
	binary.LittleEndian.PutUint64(body[24:32], 4)
	binary.LittleEndian.PutUint64(body[32:40], 5)
	body[40] = 0
	copy(body[41:73], def.Bytes())
	body[73] = 1 // is_mayhem_mode
	body[74] = 1 // is_cashback_coin
	copy(body[75:107], def.Bytes()) // quote_mint = legacy SOL sentinel

	curve, err := DecodeBondingCurve(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !curve.IsMayhemMode || !curve.IsCashbackCoin {
		t.Fatal("expected mayhem/cashback flags")
	}
	if !IsLegacyQuoteMint(curve.QuoteMint) {
		t.Fatalf("quote mint offset wrong: got %s", curve.QuoteMint)
	}
	if !CashbackEnabled(buf) {
		t.Fatal("cashback flag")
	}
}

func TestIsLegacyQuoteMint(t *testing.T) {
	if !IsLegacyQuoteMint(DefaultPubkey()) {
		t.Fatal("default pubkey should be legacy quote")
	}
	other := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	if IsLegacyQuoteMint(other) {
		t.Fatal("WSOL should not be legacy sentinel")
	}
}

func TestIsNativeSolQuotePair(t *testing.T) {
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	usdc := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	if !IsNativeSolQuotePair(solana.PublicKey{}, wsol) {
		t.Fatal("zero quote mint should be native SOL pair")
	}
	if !IsNativeSolQuotePair(DefaultPubkey(), wsol) {
		t.Fatal("default sentinel should be native SOL pair")
	}
	if !IsNativeSolQuotePair(wsol, wsol) {
		t.Fatal("WSOL quote mint should be native SOL pair")
	}
	if IsNativeSolQuotePair(usdc, wsol) {
		t.Fatal("USDC quote mint should not be native SOL pair")
	}
}
