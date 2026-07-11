package bridge

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/bridge/ammv4"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/bridge/cpmm"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/bridge/dammv2"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/logx"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
)

type Router struct {
	cfg *config.Config
}

func NewRouter(cfg *config.Config) *Router {
	return &Router{cfg: cfg}
}

type SwapBuildParams struct {
	Pool              *DiscoveredPool
	PoolAccount       *rpc.Account
	User              solana.PublicKey
	InputATA          solana.PublicKey
	OutputATA         solana.PublicKey
	AmountIn          uint64
	MinAmountOut      uint64
	MintTokenPrograms map[solana.PublicKey]solana.PublicKey
}

func (r *Router) BuildSwap(p SwapBuildParams) (solana.Instruction, error) {
	switch p.Pool.PoolType {
	case PoolRaydiumAMMv4:
		return r.buildAmmV4Swap(p)
	case PoolRaydiumCPMM:
		return r.buildCpmmSwap(p)
	case PoolMeteoraDAMMv2:
		return r.buildDammV2Swap(p)
	default:
		return nil, fmt.Errorf("bridge swap build not implemented for %s", p.Pool.PoolType)
	}
}

func (r *Router) buildAmmV4Swap(p SwapBuildParams) (solana.Instruction, error) {
	if p.PoolAccount == nil || p.PoolAccount.Data == nil {
		return nil, fmt.Errorf("amm v4 pool account missing")
	}
	state, err := ammv4.DecodePoolState(p.PoolAccount.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	programID, err := solpkg.ParsePubkey(r.cfg.Bridge.RaydiumAMMv4.ProgramID)
	if err != nil {
		return nil, err
	}
	poolPK, err := solpkg.ParsePubkey(p.Pool.PoolID)
	if err != nil {
		return nil, err
	}
	inMint, err := solpkg.ParsePubkey(p.Pool.InputMint)
	if err != nil {
		return nil, err
	}
	outMint, err := solpkg.ParsePubkey(p.Pool.OutputMint)
	if err != nil {
		return nil, err
	}
	logx.Debug("bridge", "build amm v4 swap",
		"poolId", p.Pool.PoolID,
		"coinMint", state.BaseMint.String(),
		"pcMint", state.QuoteMint.String(),
		"amountIn", p.AmountIn,
	)
	return ammv4.BuildSwapBaseInV2(ammv4.SwapParams{
		ProgramID:     programID,
		Payer:         p.User,
		Pool:          state,
		PoolID:        poolPK,
		UserInputATA:  p.InputATA,
		UserOutputATA: p.OutputATA,
		InputMint:     inMint,
		OutputMint:    outMint,
		TokenProgram:  solana.TokenProgramID,
		AmountIn:      p.AmountIn,
		MinAmountOut:  p.MinAmountOut,
	})
}

func (r *Router) buildCpmmSwap(p SwapBuildParams) (solana.Instruction, error) {
	if p.PoolAccount == nil || p.PoolAccount.Data == nil {
		return nil, fmt.Errorf("cpmm pool account missing")
	}
	state, err := cpmm.DecodePoolState(p.PoolAccount.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	programID, err := solpkg.ParsePubkey(r.cfg.Bridge.RaydiumCPMM.ProgramID)
	if err != nil {
		return nil, err
	}
	poolPK, err := solpkg.ParsePubkey(p.Pool.PoolID)
	if err != nil {
		return nil, err
	}
	inMint, err := solpkg.ParsePubkey(p.Pool.InputMint)
	if err != nil {
		return nil, err
	}
	outMint, err := solpkg.ParsePubkey(p.Pool.OutputMint)
	if err != nil {
		return nil, err
	}
	return cpmm.BuildSwapBaseInput(cpmm.SwapParams{
		ProgramID:     programID,
		Payer:         p.User,
		Pool:          state,
		PoolID:        poolPK,
		UserInputATA:  p.InputATA,
		UserOutputATA: p.OutputATA,
		InputMint:     inMint,
		OutputMint:    outMint,
		AmountIn:      p.AmountIn,
		MinAmountOut:  p.MinAmountOut,
	})
}

func mintTokenProgram(mint solana.PublicKey, programs map[solana.PublicKey]solana.PublicKey) solana.PublicKey {
	if tp, ok := programs[mint]; ok && !tp.IsZero() {
		return tp
	}
	return solana.TokenProgramID
}

func (r *Router) buildDammV2Swap(p SwapBuildParams) (solana.Instruction, error) {
	if p.PoolAccount == nil || p.PoolAccount.Data == nil {
		return nil, fmt.Errorf("damm v2 pool account missing")
	}
	state, err := dammv2.DecodePoolState(p.PoolAccount.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	programID, err := solpkg.ParsePubkey(r.cfg.Bridge.MeteoraDAMMv2.ProgramID)
	if err != nil {
		return nil, err
	}
	poolPK, err := solpkg.ParsePubkey(p.Pool.PoolID)
	if err != nil {
		return nil, err
	}
	inMint, err := solpkg.ParsePubkey(p.Pool.InputMint)
	if err != nil {
		return nil, err
	}
	outMint, err := solpkg.ParsePubkey(p.Pool.OutputMint)
	if err != nil {
		return nil, err
	}
	logx.Debug("bridge", "build damm v2 swap",
		"poolId", p.Pool.PoolID,
		"mintA", state.MintA.String(),
		"mintB", state.MintB.String(),
		"amountIn", p.AmountIn,
	)
	return dammv2.BuildSwap2(dammv2.SwapParams{
		ProgramID:     programID,
		Payer:         p.User,
		Pool:          state,
		PoolID:        poolPK,
		UserInputATA:  p.InputATA,
		UserOutputATA: p.OutputATA,
		InputMint:     inMint,
		OutputMint:    outMint,
		TokenProgramA: mintTokenProgram(state.MintA, p.MintTokenPrograms),
		TokenProgramB: mintTokenProgram(state.MintB, p.MintTokenPrograms),
		AmountIn:      p.AmountIn,
		MinAmountOut:  p.MinAmountOut,
	})
}
