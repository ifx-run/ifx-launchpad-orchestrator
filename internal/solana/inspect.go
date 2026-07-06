package solana

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go"
)

var programLabels = map[string]string{
	"ComputeBudget111111111111111111111111111111": "Compute Budget",
	"11111111111111111111111111111111":            "System Program",
	"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA": "SPL Token",
	"TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb": "Token-2022",
	"ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL": "Associated Token",
	"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P": "Pump",
	"pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ": "Pump Fee",
	"ifxmwWVVZDmXN2DUVf7wtJYCXTRY4QsL5rzmNkXzxbj": "Ifx",
}

var pumpDiscriminators = map[string]string{
	"38fc74089edfcd5f": "buy_exact_sol_in",
	"33e685a4017f83ad": "sell",
}

type TxInspection struct {
	Format               string                   `json:"format"`
	NumInstructions      int                      `json:"numInstructions"`
	StaticAccountKeys    int                      `json:"staticAccountKeys"`
	TotalAccountKeys     int                      `json:"totalAccountKeys"`
	FeePayer             string                   `json:"feePayer,omitempty"`
	RecentBlockhash      string                   `json:"recentBlockhash,omitempty"`
	TransactionSizeBytes int                      `json:"transactionSizeBytes,omitempty"`
	Instructions         []TxInstructionInspection `json:"instructions"`
}

type TxInstructionInspection struct {
	Index        int                    `json:"index"`
	ProgramID    string                 `json:"programId"`
	ProgramLabel string                 `json:"programLabel"`
	Hint         string                 `json:"hint,omitempty"`
	Accounts     []TxAccountInspection  `json:"accounts"`
	DataHex      string                 `json:"dataHex"`
	DataBase64   string                 `json:"dataBase64"`
	DataLength   int                    `json:"dataLength"`
}

type TxAccountInspection struct {
	Index      int    `json:"index"`
	Pubkey     string `json:"pubkey"`
	IsSigner   bool   `json:"isSigner"`
	IsWritable bool   `json:"isWritable"`
}

func InspectTransactionBase64(b64 string, sizeBytes int) (*TxInspection, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	tx, err := solana.TransactionFromBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("decode transaction: %w", err)
	}
	return InspectTransaction(tx, sizeBytes), nil
}

func InspectTransaction(tx *solana.Transaction, sizeBytes int) *TxInspection {
	msg := tx.Message
	if msg.NumLookups() > 0 && msg.GetAddressTables() != nil {
		_ = msg.ResolveLookups()
	}
	keys := msg.AccountKeys
	feePayer := ""
	if len(keys) > 0 {
		feePayer = keys[0].String()
	}
	if sizeBytes == 0 {
		if raw, err := tx.MarshalBinary(); err == nil {
			sizeBytes = len(raw)
		}
	}

	format := "legacy"
	if msg.GetVersion() == solana.MessageVersionV0 {
		format = "v0"
	}

	out := &TxInspection{
		Format:               format,
		NumInstructions:      len(msg.Instructions),
		StaticAccountKeys:    len(keys) - msg.NumLookups(),
		TotalAccountKeys:     len(keys),
		FeePayer:             feePayer,
		RecentBlockhash:      msg.RecentBlockhash.String(),
		TransactionSizeBytes: sizeBytes,
		Instructions:         make([]TxInstructionInspection, 0, len(msg.Instructions)),
	}

	for i, ix := range msg.Instructions {
		programID := keys[ix.ProgramIDIndex]
		data := []byte(ix.Data)
		acctMetas, _ := ix.ResolveInstructionAccounts(&msg)

		ins := TxInstructionInspection{
			Index:        i,
			ProgramID:    programID.String(),
			ProgramLabel: programLabel(programID.String()),
			Hint:         decodeInstructionHint(programID.String(), data),
			DataHex:      hex.EncodeToString(data),
			DataBase64:   base64.StdEncoding.EncodeToString(data),
			DataLength:   len(data),
			Accounts:     make([]TxAccountInspection, 0, len(acctMetas)),
		}
		for j, meta := range acctMetas {
			keyIndex := int(ix.Accounts[j])
			ins.Accounts = append(ins.Accounts, TxAccountInspection{
				Index:      keyIndex,
				Pubkey:     meta.PublicKey.String(),
				IsSigner:   meta.IsSigner,
				IsWritable: meta.IsWritable,
			})
		}
		out.Instructions = append(out.Instructions, ins)
	}
	return out
}

func programLabel(programID string) string {
	if label, ok := programLabels[programID]; ok {
		return label
	}
	if len(programID) > 8 {
		return programID[:8] + "…"
	}
	return programID
}

func decodeInstructionHint(programID string, data []byte) string {
	switch programID {
	case "ComputeBudget111111111111111111111111111111":
		if len(data) >= 5 && data[0] == 2 {
			return fmt.Sprintf("SetComputeUnitLimit(%d CU)", binary.LittleEndian.Uint32(data[1:5]))
		}
		if len(data) >= 9 && data[0] == 3 {
			return fmt.Sprintf("SetComputeUnitPrice(%d µL/CU)", binary.LittleEndian.Uint64(data[1:9]))
		}
	case "11111111111111111111111111111111":
		if len(data) >= 4 {
			switch binary.LittleEndian.Uint32(data[:4]) {
			case 2:
				return "Transfer"
			case 0:
				return "CreateAccount"
			}
		}
	case "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb":
		switch data[0] {
		case 3:
			return "Transfer"
		case 9:
			return "CloseAccount"
		case 17:
			return "SyncNative"
		case 45:
			return "UnwrapLamports"
		}
	case "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL":
		if len(data) > 0 && data[0] == 1 {
			return "CreateIdempotent"
		}
		if len(data) > 0 && data[0] == 0 {
			return "Create"
		}
	case "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P":
		if len(data) >= 8 {
			disc := hex.EncodeToString(data[:8])
			if name, ok := pumpDiscriminators[disc]; ok {
				return name
			}
			return "Pump (disc " + disc + ")"
		}
	case "ifxmwWVVZDmXN2DUVf7wtJYCXTRY4QsL5rzmNkXzxbj":
		if len(data) == 0 {
			return ""
		}
		switch data[0] {
		case 2:
			return "ixReset"
		case 3:
			return "ixLet"
		case 6:
			return "ixCpi"
		case 7:
			return "ixIfElse"
		}
	}
	return ""
}

func AcctFlags(isSigner, isWritable bool) string {
	var parts []string
	if isSigner {
		parts = append(parts, "S")
	}
	if isWritable {
		parts = append(parts, "W")
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, "")
}
