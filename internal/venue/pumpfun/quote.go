package pumpfun

import (
	"math/big"

	"github.com/gagliardetto/solana-go"
)

var (
	bpsDenom = big.NewInt(10_000)
	one      = big.NewInt(1)
)

func ceilDiv(a, b *big.Int) *big.Int {
	if b.Sign() == 0 {
		return big.NewInt(0)
	}
	num := new(big.Int).Add(a, new(big.Int).Sub(b, one))
	return num.Div(num, b)
}

func feeAmount(amount *big.Int, bps uint64) *big.Int {
	if amount.Sign() == 0 || bps == 0 {
		return big.NewInt(0)
	}
	feeBps := big.NewInt(int64(bps))
	return ceilDiv(new(big.Int).Mul(amount, feeBps), bpsDenom)
}

func protocolAndCreatorBPS(global Global, creator solana.PublicKey) (uint64, uint64) {
	protocol := global.FeeBasisPoints
	creatorBPS := uint64(0)
	if !creator.Equals(DefaultPubkey()) {
		creatorBPS = global.CreatorFeeBasisPoints
	}
	return protocol, creatorBPS
}

// BuyBaseOut estimates base token out for exact quote in (post pump protocol+creator fees).
func BuyBaseOut(global Global, curve BondingCurve, quoteIn uint64) uint64 {
	if quoteIn == 0 || curve.VirtualTokenReserves == 0 {
		return 0
	}
	amount := new(big.Int).SetUint64(quoteIn)
	protocolBPS, creatorBPS := protocolAndCreatorBPS(global, curve.Creator)
	totalFeeBPS := new(big.Int).Add(big.NewInt(int64(protocolBPS)), big.NewInt(int64(creatorBPS)))
	totalFeeBPS.Add(totalFeeBPS, bpsDenom)

	inputAmount := new(big.Int).Mul(amount, bpsDenom)
	inputAmount.Div(inputAmount, totalFeeBPS)

	vToken := new(big.Int).SetUint64(curve.VirtualTokenReserves)
	vSol := new(big.Int).SetUint64(curve.VirtualSolReserves)
	denom := new(big.Int).Add(vSol, inputAmount)
	tokens := new(big.Int).Mul(inputAmount, vToken)
	tokens.Div(tokens, denom)

	realCap := new(big.Int).SetUint64(curve.RealTokenReserves)
	if tokens.Cmp(realCap) > 0 {
		tokens = realCap
	}
	if !tokens.IsUint64() {
		return 0
	}
	return tokens.Uint64()
}

// SellQuoteOut estimates quote out for exact base in (post pump protocol+creator fees).
func SellQuoteOut(global Global, curve BondingCurve, baseIn uint64) uint64 {
	if baseIn == 0 || curve.VirtualTokenReserves == 0 {
		return 0
	}
	input := new(big.Int).SetUint64(baseIn)
	vToken := new(big.Int).SetUint64(curve.VirtualTokenReserves)
	vSol := new(big.Int).SetUint64(curve.VirtualSolReserves)

	denom := new(big.Int).Add(vToken, input)
	solOut := new(big.Int).Mul(input, vSol)
	solOut.Div(solOut, denom)

	protocolBPS, creatorBPS := protocolAndCreatorBPS(global, curve.Creator)
	protocolFee := feeAmount(solOut, protocolBPS)
	creatorFee := feeAmount(solOut, creatorBPS)
	solOut.Sub(solOut, protocolFee)
	solOut.Sub(solOut, creatorFee)

	if solOut.Sign() <= 0 || !solOut.IsUint64() {
		return 0
	}
	return solOut.Uint64()
}
