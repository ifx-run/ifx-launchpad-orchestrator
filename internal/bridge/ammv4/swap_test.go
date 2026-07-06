package ammv4_test

import (
	"testing"

	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/bridge/ammv4"
	"github.com/gagliardetto/solana-go"
)

func TestBuildSwapBaseInV2_compileValid(t *testing.T) {
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	usdt := solana.MustPublicKeyFromBase58("Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB")
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	pool := solana.MustPublicKeyFromBase58("5BKxfWMbmYBAEWvyPZS9esPducUba9GqyMjtLCfbaqyF")
	program := solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")

	userUsdt, _, _ := solana.FindAssociatedTokenAddress(user, usdt)
	userWsol, _, _ := solana.FindAssociatedTokenAddress(user, wsol)

	state := ammv4.PoolState{
		BaseVault:  solana.MustPublicKeyFromBase58("DQyrAcCrDXQ7NeoqGgDCZwBvWDcYmFCjSb9JtteuvPpz"),
		QuoteVault: solana.MustPublicKeyFromBase58("HLmqeL62xR1QoZ1HKKbXRrdN1p3phKpxRMb2VVgoDmM"),
		BaseMint:   usdt,
		QuoteMint:  wsol,
		OpenOrders: solana.MustPublicKeyFromBase58("8BnEgHoWFysVcuFFX7QztDmzuH8r5ZFvyP3sYwn1XTh6"),
	}

	ix, err := ammv4.BuildSwapBaseInV2(ammv4.SwapParams{
		ProgramID:     program,
		Payer:         user,
		Pool:          state,
		PoolID:        pool,
		UserInputATA:  userUsdt,
		UserOutputATA: userWsol,
		InputMint:     usdt,
		OutputMint:    wsol,
		AmountIn:      999_500,
		MinAmountOut:  1,
	})
	if err != nil {
		t.Fatal(err)
	}

	bh := solana.MustHashFromBase58("EkSnNWid2cvwEVnVx9aBqawnmiCNiDgp3gUdkDPTKN1N")
	_, compiled, err := solpkg.CompileV0Tx(user, bh, []solana.Instruction{ix}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.TransactionSize == 0 {
		t.Fatal("expected non-zero tx size")
	}
	if len(ix.Accounts()) != 8 {
		t.Fatalf("expected 8 accounts, got %d", len(ix.Accounts()))
	}
}
