package pumpfun

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/route"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/util"
)

// QuoteSwapAB quotes sell(A)→quote then buy(B) when both pools share the same Q_native.
func QuoteSwapAB(
	cfg *config.Config,
	accounts map[solana.PublicKey]*rpc.Account,
	mintA, mintB solana.PublicKey,
	params QuoteParams,
) (QuoteOutcome, error) {
	if params.PairClass != route.PairSwapLaunchpad {
		return QuoteOutcome{}, fmt.Errorf("quote swap: expected pair class swap_launchpad")
	}
	qA, err := QNativeFromAccounts(cfg, accounts, mintA)
	if err != nil {
		return QuoteOutcome{}, err
	}
	qB, err := QNativeFromAccounts(cfg, accounts, mintB)
	if err != nil {
		return QuoteOutcome{}, err
	}
	if qA != qB {
		return QuoteOutcome{}, fmt.Errorf("token swap requires same pool quote (A=%s B=%s); cross-quote needs bridge", qA, qB)
	}

	sellParams := params
	sellParams.PairClass = route.PairSellLaunchpad
	sellOutcome, err := QuoteFromAccounts(cfg, accounts, mintA, sellParams)
	if err != nil {
		return QuoteOutcome{}, err
	}

	bcPK, err := bondingCurvePDA(cfg, mintB)
	if err != nil {
		return QuoteOutcome{}, err
	}
	globalPK, err := GlobalPDA(cfg)
	if err != nil {
		return QuoteOutcome{}, err
	}
	bcAcct := accounts[bcPK]
	globalAcct := accounts[globalPK]
	if bcAcct == nil || globalAcct == nil {
		return QuoteOutcome{}, fmt.Errorf("bonding curve B missing from snapshot")
	}
	curveB, err := DecodeBondingCurve(bcAcct.Data.GetBinary())
	if err != nil {
		return QuoteOutcome{}, err
	}
	if curveB.Complete {
		return QuoteOutcome{}, fmt.Errorf("output token graduated (bonding curve complete)")
	}
	globalB, err := DecodeGlobal(globalAcct.Data.GetBinary())
	if err != nil {
		return QuoteOutcome{}, err
	}

	netExpected := sellOutcome.OutputAmount
	netConservative := sellOutcome.MinOutputAmount
	outExpected := BuyBaseOut(globalB, curveB, netExpected)
	outConservative := BuyBaseOut(globalB, curveB, netConservative)

	mintAcct := accounts[mintB]
	if mintAcct == nil {
		return QuoteOutcome{}, fmt.Errorf("output mint missing from snapshot")
	}
	baseDecimals, err := MintDecimals(mintAcct.Data.GetBinary())
	if err != nil {
		return QuoteOutcome{}, err
	}

	return QuoteOutcome{
		OutputAmount:      outExpected,
		MinOutputAmount:   util.MinOut(outConservative, params.SlippageBPS),
		GrossOutputAmount: sellOutcome.GrossOutputAmount,
		ServiceFeeAmount:  sellOutcome.ServiceFeeAmount,
		QNative:           qA,
		BaseDecimals:      baseDecimals,
	}, nil
}
