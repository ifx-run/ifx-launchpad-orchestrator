package ifx_test

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	ifxpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/ifx"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/venue/pumpfun"
)

func testCfg() *config.Config {
	return &config.Config{
		Ifx: config.IfxConfig{
			ProgramID:    "ifxmwWVVZDmXN2DUVf7wtJYCXTRY4QsL5rzmNkXzxbj",
			PublicFrames: []string{"6RNv1eQ7fogEW7R1QGg6dAiddEefGfYgJVtjpvgENtdn"},
		},
		ServiceFee: config.ServiceFeeConfig{BPS: 5},
	}
}

func TestPlanPumpSellThenBuySPL_includesIfxChain(t *testing.T) {
	cfg := testCfg()
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	sellTpl := solana.NewInstruction(pumpfun.ProgramID(), solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, make([]byte, 24))
	buyTpl := solana.NewInstruction(pumpfun.ProgramID(), solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, make([]byte, 24))

	ixs, err := ifxpkg.PlanPumpSellThenBuy(cfg, ifxpkg.SellThenBuyParams{
		QuoteKind:           pumpfun.QuoteSPL,
		SellTemplate:        sellTpl,
		BuyTemplate:         buyTpl,
		ServiceFeeBPS:       cfg.ServiceFee.BPS,
		User:                user,
		PlatformFeePubkey:   user,
		PlatformFeeQuoteATA: user,
		QuoteATA:            user,
		QuoteMint:           user,
		QuoteTokenProgram:   solana.TokenProgramID,
		QuoteDecimals:       6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) < 5 {
		t.Fatalf("expected ifx sell→buy chain, got %d instructions", len(ixs))
	}
	assertIfxReset(t, cfg, ixs[0])
	assertIfxCpi(t, cfg, ixs[len(ixs)-1])
}

func TestPlanPumpSellThenBridgeSPL_zeroFeeStillUsesIfx(t *testing.T) {
	cfg := testCfg()
	cfg.ServiceFee.BPS = 0
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	sellTpl := solana.NewInstruction(pumpfun.ProgramID(), solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, make([]byte, 24))
	bridgeTpl := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, make([]byte, 17))

	ixs, err := ifxpkg.PlanPumpSellThenBridge(cfg, ifxpkg.SellThenBridgeParams{
		QuoteKind:            pumpfun.QuoteSPL,
		SellTemplate:         sellTpl,
		BridgeTemplate:       bridgeTpl,
		BridgeAmountInOffset: 8,
		ServiceFeeBPS:        0,
		User:                 user,
		QuoteATA:             user,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) < 4 {
		t.Fatalf("expected ifx chain without fee legs, got %d", len(ixs))
	}
	assertIfxReset(t, cfg, ixs[0])
	assertIfxCpi(t, cfg, ixs[len(ixs)-1])
}

func TestPlanBridgeThenPumpBuySponsored_includesRepayChain(t *testing.T) {
	cfg := testCfg()
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	bridgeSwap := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, []byte{0})
	buyTpl := solana.NewInstruction(pumpfun.ProgramID(), solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, make([]byte, 24))
	unwrapRaw := solana.NewInstruction(solana.TokenProgramID, solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, []byte{9})
	var unwrapIx solana.Instruction = unwrapRaw

	ixs, err := ifxpkg.PlanBridgeThenPumpBuySponsored(cfg, ifxpkg.BridgeThenPumpBuyParams{
		BridgeSwap:      bridgeSwap,
		MeasureQuoteATA: user,
		BuyTemplate:     buyTpl,
		UnwrapWSOL:      &unwrapIx,
	}, ifxpkg.SponsoredRepayParams{User: user, RepayTo: user}, 5000, user, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) < 6 {
		t.Fatalf("expected sponsored bridge→buy chain, got %d instructions", len(ixs))
	}
	assertIfxReset(t, cfg, ixs[0])
	assertIfxCpi(t, cfg, ixs[len(ixs)-1])
}

func TestPlanBridgeThenPumpBuy_includesIfxChain(t *testing.T) {
	cfg := testCfg()
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	bridgeSwap := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, []byte{0})
	buyTpl := solana.NewInstruction(pumpfun.ProgramID(), solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, make([]byte, 24))

	ixs, err := ifxpkg.PlanBridgeThenPumpBuy(cfg, ifxpkg.BridgeThenPumpBuyParams{
		BridgeSwap:      bridgeSwap,
		MeasureQuoteATA: user,
		BuyTemplate:     buyTpl,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) < 4 {
		t.Fatalf("expected ifx buy chain, got %d instructions", len(ixs))
	}
	assertIfxReset(t, cfg, ixs[0])
	assertIfxCpi(t, cfg, ixs[len(ixs)-1])
}

func TestPlanPumpSellThenBridgeSPL_includesIfxChain(t *testing.T) {
	cfg := testCfg()
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	sellTpl := solana.NewInstruction(pumpfun.ProgramID(), solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, make([]byte, 24))
	bridgeTpl := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, make([]byte, 17))

	ixs, err := ifxpkg.PlanPumpSellThenBridge(cfg, ifxpkg.SellThenBridgeParams{
		QuoteKind:            pumpfun.QuoteSPL,
		SellTemplate:         sellTpl,
		BridgeTemplate:       bridgeTpl,
		BridgeAmountInOffset: 8,
		ServiceFeeBPS:        cfg.ServiceFee.BPS,
		User:                 user,
		PlatformFeePubkey:    user,
		PlatformFeeQuoteATA:  user,
		QuoteATA:             user,
		QuoteMint:            user,
		QuoteTokenProgram:    solana.TokenProgramID,
		QuoteDecimals:        6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) < 5 {
		t.Fatalf("expected ifx sell→bridge chain, got %d instructions", len(ixs))
	}
	assertIfxReset(t, cfg, ixs[0])
	assertIfxCpi(t, cfg, ixs[len(ixs)-1])
}

func assertIfxReset(t *testing.T, cfg *config.Config, ix solana.Instruction) {
	t.Helper()
	data, err := ix.Data()
	if err != nil {
		t.Fatal(err)
	}
	if ix.ProgramID().String() != cfg.Ifx.ProgramID || len(data) == 0 || data[0] != 2 {
		t.Fatalf("expected ifx reset, got program=%s disc=%v", ix.ProgramID(), data)
	}
}

func assertIfxCpi(t *testing.T, cfg *config.Config, ix solana.Instruction) {
	t.Helper()
	data, err := ix.Data()
	if err != nil {
		t.Fatal(err)
	}
	if ix.ProgramID().String() != cfg.Ifx.ProgramID || len(data) == 0 || data[0] != 6 {
		t.Fatalf("expected ifx cpi, got program=%s disc=%v", ix.ProgramID(), data)
	}
}

func TestPlanQuoteBridgeSponsored_includesRepayChain(t *testing.T) {
	cfg := testCfg()
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	bridgeSwap := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, []byte{0})
	unwrapRaw := solana.NewInstruction(solana.TokenProgramID, solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, []byte{9})
	var unwrapIx solana.Instruction = unwrapRaw

	ixs, err := ifxpkg.PlanQuoteBridgeSponsored(cfg, ifxpkg.QuoteBridgeParams{
		BridgeSwap: bridgeSwap,
		UnwrapWSOL: &unwrapIx,
		User:       user,
	}, ifxpkg.SponsoredRepayParams{User: user, RepayTo: user}, 5000, user, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) < 5 {
		t.Fatalf("expected sponsored quote bridge chain, got %d instructions", len(ixs))
	}
	assertIfxReset(t, cfg, ixs[0])
	assertIfxCpi(t, cfg, ixs[len(ixs)-1])
}

func TestPlanQuoteBridgeSponsored_partialUnwrapRepay(t *testing.T) {
	cfg := testCfg()
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	wsolATA := solana.MustPublicKeyFromBase58("EoMLb17Wkmys4UTtU82ZAYr6ysuPf1D4DEVq4QZokGFH")
	bridgeSwap := solana.NewInstruction(solana.SystemProgramID, solana.AccountMetaSlice{
		{PublicKey: user, IsSigner: true, IsWritable: true},
	}, []byte{0})

	ixs, err := ifxpkg.PlanQuoteBridgeSponsored(cfg, ifxpkg.QuoteBridgeParams{
		BridgeSwap:   bridgeSwap,
		WSOLATA:      wsolATA,
		User:         user,
		TokenProgram: solana.TokenProgramID,
		RepayPartial: true,
	}, ifxpkg.SponsoredRepayParams{User: user, RepayTo: user}, 5000, user, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) < 6 {
		t.Fatalf("expected partial unwrap sponsored chain, got %d instructions", len(ixs))
	}
	assertIfxReset(t, cfg, ixs[0])
	assertIfxCpi(t, cfg, ixs[len(ixs)-1])
}
