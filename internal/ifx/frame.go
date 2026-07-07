package ifx

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
	"github.com/ifx-run/ifx/go-sdk/scratch"
)

const DefaultTapeLen = 512

func NewScratch(cfg *config.Config) (*scratch.FrameScratch, error) {
	if len(cfg.Ifx.PublicFrames) == 0 {
		return nil, fmt.Errorf("ifx.public_frames is required for Ifx-orchestrated builds")
	}
	framePK, err := solpkg.ParsePubkey(cfg.Ifx.PublicFrames[0])
	if err != nil {
		return nil, err
	}
	programID, err := solpkg.ParsePubkey(cfg.Ifx.ProgramID)
	if err != nil {
		return nil, err
	}
	tapeLen := DefaultTapeLen
	return scratch.ForPublicFrame(framePK, programID, &tapeLen), nil
}

func ProgramID(cfg *config.Config) (solana.PublicKey, error) {
	return solpkg.ParsePubkey(cfg.Ifx.ProgramID)
}
