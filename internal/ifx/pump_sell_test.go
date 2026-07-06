package ifx_test

import (
	"testing"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	ifxpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/ifx"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/gagliardetto/solana-go"
)

func TestPlanPumpSellWithSOLFee_includesIfxInstructions(t *testing.T) {
	cfg := &config.Config{
		Ifx: config.IfxConfig{
			ProgramID:    "ifxmwWVVZDmXN2DUVf7wtJYCXTRY4QsL5rzmNkXzxbj",
			PublicFrames: []string{"6RNv1eQ7fogEW7R1QGg6dAiddEefGfYgJVtjpvgENtdn"},
		},
		ServiceFee: config.ServiceFeeConfig{BPS: 5},
	}
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	mint := solana.MustPublicKeyFromBase58("CzLSujWBLFsSjncfkh59rUFqvafWcY5tzedWJSuypump")
	ixs, err := ifxpkg.PlanPumpSellWithSOLFee(cfg, pumpfun.BuildParams{
		Curve: pumpfun.BondingCurve{
			VirtualTokenReserves: 1_073_000_000_000_000,
			VirtualSolReserves:   30_000_000_000,
			RealTokenReserves:    793_100_000_000_000,
			TokenTotalSupply:     1_000_000_000_000_000,
			Creator:              solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
		},
		BaseMint:          mint,
		User:              user,
		BaseTokenProgram:  solana.TokenProgramID,
		BaseAmountIn:      1_000_000,
		MinQuoteOut:       1,
		PlatformFeePubkey: solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
		ComputeUnitLimit:  500_000,
		ComputeUnitPrice:  10_000,
	}, cfg.ServiceFee.BPS)
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) < 5 {
		t.Fatalf("expected ifx sell ix chain, got %d instructions", len(ixs))
	}
	ifxProgram := cfg.Ifx.ProgramID
	resetData, err := ixs[0].Data()
	if err != nil {
		t.Fatal(err)
	}
	if ixs[0].ProgramID().String() != ifxProgram || len(resetData) == 0 || resetData[0] != 2 {
		t.Fatalf("first ix should be ifx reset, got program=%s disc=%v", ixs[0].ProgramID(), resetData)
	}
	last := ixs[len(ixs)-1]
	lastData, err := last.Data()
	if err != nil {
		t.Fatal(err)
	}
	if last.ProgramID().String() != ifxProgram || len(lastData) == 0 || lastData[0] != 6 {
		t.Fatalf("last ix should be ifx cpi, got program=%s disc=%v", last.ProgramID(), lastData)
	}
	_, compiled, err := solpkg.CompileV0Tx(user, solana.MustHashFromBase58("EkSnNWid2cvwEVnVx9aBqawnmiCNiDgp3gUdkDPTKN1N"), ixs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.TransactionSize == 0 {
		t.Fatal("empty transaction")
	}
}

func TestPlanPumpBuy_noIfxReset(t *testing.T) {
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	mint := solana.MustPublicKeyFromBase58("CzLSujWBLFsSjncfkh59rUFqvafWcY5tzedWJSuypump")
	ixs, err := pumpfun.BuildBuyInstructions(pumpfun.BuildParams{
		Curve: pumpfun.BondingCurve{
			VirtualTokenReserves: 1_073_000_000_000_000,
			VirtualSolReserves:   30_000_000_000,
			RealTokenReserves:    793_100_000_000_000,
			TokenTotalSupply:     1_000_000_000_000_000,
			Creator:              solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
		},
		BaseMint: mint, User: user, BaseTokenProgram: solana.TokenProgramID,
		SpendableQuoteIn: 10_000_000, MinBaseOut: 1, ServiceFeeLamports: 100_000,
		PlatformFeePubkey: solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM"),
		ComputeUnitLimit: 200_000, ComputeUnitPrice: 1_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	ifxProgram := "ifxmwWVVZDmXN2DUVf7wtJYCXTRY4QsL5rzmNkXzxbj"
	for _, ix := range ixs {
		if ix.ProgramID().String() == ifxProgram {
			t.Fatal("buy path must not include Ifx instructions")
		}
	}
}
