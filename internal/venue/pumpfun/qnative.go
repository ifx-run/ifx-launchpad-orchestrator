package pumpfun

import (
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// QNativeFromAccounts reads the pool-native quote mint for a bonding curve.
func QNativeFromAccounts(cfg *config.Config, accounts map[solana.PublicKey]*rpc.Account, baseMint solana.PublicKey) (string, error) {
	bcPK, err := bondingCurvePDA(cfg, baseMint)
	if err != nil {
		return "", err
	}
	bcAcct, ok := accounts[bcPK]
	if !ok || bcAcct == nil || bcAcct.Data == nil {
		return "", fmt.Errorf("bonding curve account missing from snapshot")
	}
	curve, err := DecodeBondingCurve(bcAcct.Data.GetBinary())
	if err != nil {
		return "", err
	}
	wsol, _ := solana.PublicKeyFromBase58(cfg.Quotes.WSOLMint)
	return EffectiveQuoteMint(cfg, curve.QuoteMint, wsol).String(), nil
}
