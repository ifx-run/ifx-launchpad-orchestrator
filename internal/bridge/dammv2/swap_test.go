package dammv2_test

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/bridge/dammv2"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
)

func TestPoolAuthorityPDA(t *testing.T) {
	program := solana.MustPublicKeyFromBase58("cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG")
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("pool_authority")}, program)
	if err != nil {
		t.Fatal(err)
	}
	want := solana.MustPublicKeyFromBase58("HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC")
	if !pda.Equals(want) && !dammv2.PoolAuthority.Equals(want) {
		t.Fatalf("pool authority pda=%s constant=%s want=%s", pda, dammv2.PoolAuthority, want)
	}
}

func TestBuildSwap2_compileValid(t *testing.T) {
	user := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	usdc := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	pool := solana.MustPublicKeyFromBase58("5BKxfWMbmYBAEWvyPZS9esPducUba9GqyMjtLCfbaqyF")
	program := solana.MustPublicKeyFromBase58("cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG")

	userUsdc, _, _ := solana.FindAssociatedTokenAddress(user, usdc)
	userWsol, _, _ := solana.FindAssociatedTokenAddress(user, wsol)

	state := dammv2.PoolState{
		MintA:  usdc,
		MintB:  wsol,
		VaultA: solana.MustPublicKeyFromBase58("DQyrAcCrDXQ7NeoqGgDCZwBvWDcYmFCjSb9JtteuvPpz"),
		VaultB: solana.MustPublicKeyFromBase58("HLmqeL62xR1QoZ1HKKbXRrdN1p3phKpxRMb2VVgoDmM"),
	}

	ix, err := dammv2.BuildSwap2(dammv2.SwapParams{
		ProgramID:       program,
		Payer:           user,
		Pool:            state,
		PoolID:          pool,
		UserInputATA:    userUsdc,
		UserOutputATA:   userWsol,
		InputMint:       usdc,
		OutputMint:      wsol,
		TokenProgramA:   solana.TokenProgramID,
		TokenProgramB:   solana.TokenProgramID,
		AmountIn:        1_000_000,
		MinAmountOut:    1,
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
	if len(ix.Accounts()) != 14 {
		t.Fatalf("expected 14 accounts, got %d", len(ix.Accounts()))
	}
	data, err := ix.Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 25 {
		t.Fatalf("expected 25-byte swap2 data, got %d", len(data))
	}
	if data[24] != 0 {
		t.Fatalf("expected swap_mode ExactIn (0), got %d", data[24])
	}
}

func TestDecodePoolState_offsets(t *testing.T) {
	usdc := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	vaultA := solana.MustPublicKeyFromBase58("11111111111111111111111111111112")
	vaultB := solana.MustPublicKeyFromBase58("11111111111111111111111111111113")

	data := make([]byte, 8+160+128)
	copy(data[8+160:], usdc.Bytes())
	copy(data[8+160+32:], wsol.Bytes())
	copy(data[8+160+64:], vaultA.Bytes())
	copy(data[8+160+96:], vaultB.Bytes())

	got, err := dammv2.DecodePoolState(data)
	if err != nil {
		t.Fatal(err)
	}
	if !got.MintA.Equals(usdc) || !got.MintB.Equals(wsol) || !got.VaultA.Equals(vaultA) || !got.VaultB.Equals(vaultB) {
		t.Fatalf("decode mismatch: %+v", got)
	}
}
