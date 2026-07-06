package orchestrator

import (
	"context"

	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
)

func (s *Service) InspectTransaction(b64 string) (*solpkg.TxInspection, error) {
	return solpkg.InspectTransactionBase64(b64, 0)
}

func (s *Service) SimulateTransaction(ctx context.Context, b64 string) (*solpkg.TxSimulation, error) {
	return s.solana.SimulateTransactionBase64(ctx, b64)
}
