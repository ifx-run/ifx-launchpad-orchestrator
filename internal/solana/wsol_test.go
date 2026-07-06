package solana_test

import (
	"encoding/binary"
	"testing"

	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/gagliardetto/solana-go"
)

func TestUnwrapWSOLInstructions_partialUsesUnwrapLamports(t *testing.T) {
	owner := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	ixs, err := solpkg.UnwrapWSOLInstructions(owner, wsol, solana.TokenProgramID, 10_000_000, solpkg.WSOLUnwrapPartial)
	if err != nil {
		t.Fatal(err)
	}
	if len(ixs) != 2 {
		t.Fatalf("want 2 ixs, got %d", len(ixs))
	}
	data, err := ixs[1].Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 10 || data[0] != 45 || data[1] != 1 {
		t.Fatalf("unwrap data=%v", data)
	}
	if got := binary.LittleEndian.Uint64(data[2:10]); got != 10_000_000 {
		t.Fatalf("amount=%d", got)
	}
}

func TestUnwrapWSOLInstructions_closeUsesCloseAccount(t *testing.T) {
	owner := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	ixs, err := solpkg.UnwrapWSOLInstructions(owner, wsol, solana.TokenProgramID, 10_000_000, solpkg.WSOLUnwrapClose)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ixs[1].Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data[0] != 9 {
		t.Fatalf("close data=%v", data)
	}
}

func TestUnwrapWSOLInstructions_allUsesUnwrapLamportsNone(t *testing.T) {
	owner := solana.MustPublicKeyFromBase58("BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P")
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	ixs, err := solpkg.UnwrapWSOLInstructions(owner, wsol, solana.TokenProgramID, 10_000_000, solpkg.WSOLUnwrapLamportsAll)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ixs[1].Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 2 || data[0] != 45 || data[1] != 0 {
		t.Fatalf("unwrap all data=%v", data)
	}
}
