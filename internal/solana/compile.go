package solana

import (
	"encoding/base64"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

type CompiledTx struct {
	Transaction     string `json:"transaction"`
	RecentBlockhash string `json:"recentBlockhash"`
	FeePayer        string `json:"feePayer"`
	TransactionSize int    `json:"transactionSizeBytes"`
}

func CompileV0Tx(
	payer solana.PublicKey,
	blockhash solana.Hash,
	instructions []solana.Instruction,
	alts map[solana.PublicKey]solana.PublicKeySlice,
) (*solana.Transaction, CompiledTx, error) {
	if len(instructions) == 0 {
		return nil, CompiledTx{}, fmt.Errorf("requires at least one instruction")
	}
	opts := []solana.TransactionOption{solana.TransactionPayer(payer)}
	if len(alts) > 0 {
		opts = append(opts, solana.TransactionAddressTables(alts))
	}
	tx, err := solana.NewTransaction(instructions, blockhash, opts...)
	if err != nil {
		return nil, CompiledTx{}, fmt.Errorf("new transaction: %w", err)
	}
	if tx.Message.GetVersion() == solana.MessageVersionLegacy {
		tx.Message.SetVersion(solana.MessageVersionV0)
	}
	required := int(tx.Message.Header.NumRequiredSignatures)
	if len(tx.Signatures) < required {
		tx.Signatures = make([]solana.Signature, required)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		return nil, CompiledTx{}, fmt.Errorf("marshal transaction: %w", err)
	}
	return tx, CompiledTx{
		Transaction:     base64.StdEncoding.EncodeToString(raw),
		RecentBlockhash: blockhash.String(),
		FeePayer:        payer.String(),
		TransactionSize: len(raw),
	}, nil
}
