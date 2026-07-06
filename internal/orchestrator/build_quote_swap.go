package orchestrator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/bridge"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/util"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func (s *Service) buildQuoteSwap(
	ctx context.Context,
	in QuoteInput,
	pool *bridge.DiscoveredPool,
	accounts map[solana.PublicKey]*rpc.Account,
) (*VariantsResult, error) {
	user, err := solpkg.ParsePubkey(in.UserPubkey)
	if err != nil {
		return nil, err
	}
	tier := s.cfg.Tier(in.PriorityTier)

	inMint, err := solpkg.ParsePubkey(in.InputMint)
	if err != nil {
		return nil, err
	}
	outMint, err := solpkg.ParsePubkey(in.OutputMint)
	if err != nil {
		return nil, err
	}
	inDec := quoteDecimals(s.cfg, in.InputMint)
	outDec := quoteDecimals(s.cfg, in.OutputMint)
	inputRaw, err := util.ResolveInputAmount(in.InputAmount, in.InputAmountRaw, inDec)
	if err != nil {
		return nil, err
	}
	outRaw, err := strconv.ParseUint(pool.OutAmount, 10, 64)
	if err != nil {
		return nil, err
	}
	minOut := util.MinOut(outRaw, in.SlippageBPS)

	poolPK, err := solpkg.ParsePubkey(pool.PoolID)
	if err != nil {
		return nil, err
	}
	poolAcct := accounts[poolPK]
	if poolAcct == nil {
		return nil, fmt.Errorf("snapshot missing bridge pool %s", pool.PoolID)
	}

	inPair, err := solpkg.DeriveATAPair(user, inMint)
	if err != nil {
		return nil, err
	}
	outPair, err := solpkg.DeriveATAPair(user, outMint)
	if err != nil {
		return nil, err
	}
	inMintAcct := accounts[inMint]
	if inMintAcct == nil {
		return nil, fmt.Errorf("snapshot missing input mint %s", in.InputMint)
	}
	outMintAcct := accounts[outMint]
	if outMintAcct == nil {
		return nil, fmt.Errorf("snapshot missing output mint %s", in.OutputMint)
	}
	userInATA := solpkg.SelectATA(inPair, inMintAcct.Owner)
	userOutATA := solpkg.SelectATA(outPair, outMintAcct.Owner)

	ata := newATASetup()
	if err := ata.ensure(user, user, inMint, inMintAcct.Owner); err != nil {
		return nil, err
	}
	if err := ata.ensure(user, user, outMint, outMintAcct.Owner); err != nil {
		return nil, err
	}
	wsolMint := solana.MustPublicKeyFromBase58(s.cfg.Quotes.WSOLMint)
	wrapBridgeSOL := shouldWrapSOLForBridge(in, inMint, wsolMint, inputRaw)
	if wrapBridgeSOL {
		if err := ata.ensure(user, user, wsolMint, solana.TokenProgramID); err != nil {
			return nil, err
		}
	}

	var ixs []solana.Instruction
	if err := ata.appendTo(&ixs, user); err != nil {
		return nil, err
	}
	if wrapBridgeSOL {
		if err := appendWrapSOLDeposit(&ixs, user, user, wsolMint, solana.TokenProgramID, inputRaw); err != nil {
			return nil, err
		}
	}

	router := bridge.NewRouter(s.cfg)
	swapIx, err := router.BuildSwap(bridge.SwapBuildParams{
		Pool:         pool,
		PoolAccount:  poolAcct,
		User:         user,
		InputATA:     userInATA,
		OutputATA:    userOutATA,
		AmountIn:     inputRaw,
		MinAmountOut: minOut,
	})
	if err != nil {
		return nil, err
	}
	ixs = append(ixs, swapIx)

	if err := s.appendUnwrapWSOLIfNeeded(&ixs, in.OutputSettlement, outMint, user, wsolMint); err != nil {
		return nil, err
	}

	_ = outDec
	legs := []route.Leg{{Kind: route.LegQuoteBridge, InputMint: in.InputMint, OutputMint: in.OutputMint}}
	return s.compileVariantsFromIXs(ctx, in, user, tier, ixs, 0, legs, false, ata.count())
}

func quoteDecimals(cfg *config.Config, mint string) uint8 {
	if mint == cfg.Quotes.WSOLMint {
		return 9
	}
	return 6
}
