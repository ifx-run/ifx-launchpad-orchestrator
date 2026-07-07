//go:build integration

package solana_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func TestPumpBuyTx_simulateRPC(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "config.toml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Skip("config.toml missing")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	client := solpkg.NewClient(cfg)

	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	mint := solana.MustPublicKeyFromBase58("CzLSujWBLFsSjncfkh59rUFqvafWcY5tzedWJSuypump")
	bcPK, err := pumpfun.BondingCurvePDAFromMint(mint)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	bcAcct, err := client.RPC().GetAccountInfoWithOpts(ctx, bcPK, &rpc.GetAccountInfoOpts{
		Commitment: client.Commitment(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if bcAcct == nil || bcAcct.Value == nil {
		t.Fatal("bonding curve account missing")
	}
	curve, err := pumpfun.DecodeBondingCurve(bcAcct.Value.Data.GetBinary())
	if err != nil {
		t.Fatal(err)
	}
	if curve.Complete {
		t.Skip("mint graduated")
	}

	ixs, err := pumpfun.BuildBuyInstructions(pumpfun.BuildParams{
		Curve: curve, BaseMint: mint, User: user,
		BaseTokenProgram: solana.TokenProgramID,
		SpendableQuoteIn: 10_000_000, MinBaseOut: 1, ServiceFeeLamports: 100_000,
		PlatformFeePubkey: solana.MustPublicKeyFromBase58(cfg.ServiceFee.Pubkey),
		ComputeUnitLimit:  200_000, ComputeUnitPrice: 1_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	bh, err := client.LatestBlockhash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	blockhash, err := solana.HashFromBase58(bh.Hash)
	if err != nil {
		t.Fatal(err)
	}
	_, compiled, err := solpkg.CompileV0Tx(user, blockhash, ixs, nil)
	if err != nil {
		t.Fatal(err)
	}

	sim, err := client.SimulateTransactionBase64(ctx, compiled.Transaction)
	if err != nil {
		if strings.Contains(err.Error(), "sanitize accounts offsets") {
			t.Fatalf("transaction still fails RPC sanitize: %v", err)
		}
		t.Fatalf("simulate rpc error: %v", err)
	}
	if !sim.Ok {
		t.Fatalf("simulate program error: %s logs=%v", sim.Error, sim.Logs)
	}
}
