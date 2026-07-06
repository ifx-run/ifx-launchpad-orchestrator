package pumpfun

import (
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/util"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type QuoteParams struct {
	PairClass      route.PairClass
	InputMint      string
	OutputMint     string
	InputAmount    string
	InputAmountRaw string
	SlippageBPS    int
	ServiceFeeBPS  uint16
}

type QuoteOutcome struct {
	OutputAmount      uint64
	MinOutputAmount   uint64
	GrossOutputAmount uint64 // sell: pump quote before platform fee
	ServiceFeeAmount  uint64
	QNative           string
	BaseDecimals      uint8
}

func QuoteFromAccounts(cfg *config.Config, accounts map[solana.PublicKey]*rpc.Account, baseMint solana.PublicKey, params QuoteParams) (QuoteOutcome, error) {
	bcPK, err := bondingCurvePDA(cfg, baseMint)
	if err != nil {
		return QuoteOutcome{}, err
	}
	globalPK, err := GlobalPDA(cfg)
	if err != nil {
		return QuoteOutcome{}, err
	}

	bcAcct, ok := accounts[bcPK]
	if !ok || bcAcct == nil || bcAcct.Data == nil {
		return QuoteOutcome{}, fmt.Errorf("bonding curve account missing from snapshot")
	}
	globalAcct, ok := accounts[globalPK]
	if !ok || globalAcct == nil || globalAcct.Data == nil {
		return QuoteOutcome{}, fmt.Errorf("pump global account missing from snapshot")
	}

	curve, err := DecodeBondingCurve(bcAcct.Data.GetBinary())
	if err != nil {
		return QuoteOutcome{}, err
	}
	if curve.Complete {
		return QuoteOutcome{}, fmt.Errorf("token graduated (bonding curve complete)")
	}

	global, err := DecodeGlobal(globalAcct.Data.GetBinary())
	if err != nil {
		return QuoteOutcome{}, err
	}

	wsol, _ := solana.PublicKeyFromBase58(cfg.Quotes.WSOLMint)
	quoteMint := EffectiveQuoteMint(cfg, curve.QuoteMint, wsol)
	mintAcct, ok := accounts[baseMint]
	if !ok || mintAcct == nil || mintAcct.Data == nil {
		return QuoteOutcome{}, fmt.Errorf("base mint missing from snapshot")
	}
	baseDecimals, err := MintDecimals(mintAcct.Data.GetBinary())
	if err != nil {
		return QuoteOutcome{}, err
	}

	switch params.PairClass {
	case route.PairBuyLaunchpad:
		return quoteBuy(cfg, global, curve, quoteMint, baseDecimals, params)
	case route.PairSellLaunchpad:
		return quoteSell(cfg, global, curve, quoteMint, baseDecimals, params)
	default:
		return QuoteOutcome{}, fmt.Errorf("pump quote: unsupported pair class %s", params.PairClass)
	}
}

func quoteBuy(cfg *config.Config, global Global, curve BondingCurve, quoteMint solana.PublicKey, baseDecimals uint8, params QuoteParams) (QuoteOutcome, error) {
	quoteDecimals := quoteDecimalsFor(cfg, quoteMint)
	inputRaw, err := util.ResolveInputAmount(params.InputAmount, params.InputAmountRaw, quoteDecimals)
	if err != nil {
		return QuoteOutcome{}, err
	}
	fee := util.ApplyBPS(inputRaw, params.ServiceFeeBPS)
	if fee >= inputRaw {
		return QuoteOutcome{}, fmt.Errorf("input too small after service fee")
	}
	netQuote := inputRaw - fee
	out := BuyBaseOut(global, curve, netQuote)
	return QuoteOutcome{
		OutputAmount:     out,
		MinOutputAmount:  util.MinOut(out, params.SlippageBPS),
		ServiceFeeAmount: fee,
		QNative:          quoteMint.String(),
		BaseDecimals:     baseDecimals,
	}, nil
}

func quoteSell(cfg *config.Config, global Global, curve BondingCurve, quoteMint solana.PublicKey, baseDecimals uint8, params QuoteParams) (QuoteOutcome, error) {
	inputRaw, err := util.ResolveInputAmount(params.InputAmount, params.InputAmountRaw, baseDecimals)
	if err != nil {
		return QuoteOutcome{}, err
	}
	grossQuote := SellQuoteOut(global, curve, inputRaw)
	minGross := util.MinOut(grossQuote, params.SlippageBPS)
	// Conservative: platform fee on slippage-adjusted gross so post-sell transfer won't exceed proceeds.
	fee := util.ApplyBPS(minGross, params.ServiceFeeBPS)
	if fee >= minGross {
		return QuoteOutcome{}, fmt.Errorf("output too small after service fee")
	}
	expectedFee := util.ApplyBPS(grossQuote, params.ServiceFeeBPS)
	return QuoteOutcome{
		OutputAmount:      grossQuote - expectedFee,
		MinOutputAmount:   minGross - fee,
		GrossOutputAmount: grossQuote,
		ServiceFeeAmount:  fee,
		QNative:           quoteMint.String(),
		BaseDecimals:      baseDecimals,
	}, nil
}

// EffectiveQuoteMint maps on-chain curve quote sentinel to the mint used in v2 ix.
func EffectiveQuoteMint(cfg *config.Config, onChain, wsolMint solana.PublicKey) solana.PublicKey {
	if IsNativeSolQuotePair(onChain, wsolMint) {
		return wsolMint
	}
	return onChain
}

func quoteDecimalsFor(cfg *config.Config, quoteMint solana.PublicKey) uint8 {
	if quoteMint.String() == cfg.Quotes.WSOLMint {
		return 9
	}
	return 6
}

func bondingCurvePDA(cfg *config.Config, baseMint solana.PublicKey) (solana.PublicKey, error) {
	program, err := solana.PublicKeyFromBase58(cfg.Venues.Pump.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("bonding-curve"), baseMint.Bytes()}, program)
	return pda, err
}
