package solana

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type TxSimulation struct {
	Ok            bool     `json:"ok"`
	Error         string   `json:"error,omitempty"`
	Logs          []string `json:"logs,omitempty"`
	UnitsConsumed *uint64  `json:"unitsConsumed,omitempty"`
}

func (c *Client) SimulateTransactionBase64(ctx context.Context, b64 string) (*TxSimulation, error) {
	tx, err := solana.TransactionFromBase64(b64)
	if err != nil {
		return nil, fmt.Errorf("decode transaction: %w", err)
	}
	return c.SimulateTransaction(ctx, tx)
}

func (c *Client) SimulateTransaction(ctx context.Context, tx *solana.Transaction) (*TxSimulation, error) {
	resp, err := c.rpc.SimulateTransactionWithOpts(ctx, tx, &rpc.SimulateTransactionOpts{
		ReplaceRecentBlockhash: true,
		Commitment:             c.commitment,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Value == nil {
		return &TxSimulation{Ok: false, Error: "empty simulate response"}, nil
	}
	result := &TxSimulation{
		Logs:          resp.Value.Logs,
		UnitsConsumed: resp.Value.UnitsConsumed,
	}
	if resp.Value.Err != nil {
		result.Ok = false
		if b, err := json.Marshal(resp.Value.Err); err == nil {
			result.Error = string(b)
		} else {
			result.Error = fmt.Sprint(resp.Value.Err)
		}
		return result, nil
	}
	result.Ok = true
	return result, nil
}
