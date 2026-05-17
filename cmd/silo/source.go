package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/curated"
	"github.com/nicolasperalta/silo2/internal/engram"
)

// identitySource is the resolved set of observations used to build an
// Identity, plus traceability about where they came from. Both
// `silo profile` and `silo outputs` go through this helper so the
// "Curated wins when useful, Engram is the fallback" policy lives in
// exactly one place.
type identitySource struct {
	Observations []engram.Observation
	// Origin is "curated" or "raw/engram". Stable string on purpose:
	// markdown templates embed it in frontmatter for traceability and
	// tests assert on it.
	Origin string
	// CLILabel is the human-readable line the CLI prints, e.g.
	// "source: curated (3 notes)" or "source: raw/engram fallback (2 observations)".
	CLILabel string
}

// loadIdentitySource decides which observation set to feed identity-building.
//
// Policy (mirrors `silo profile`):
//
//  1. Read vault/Curated/ via curated.LoadCurated. If at least one note
//     contains useful human prose, use those synthetic observations.
//  2. Otherwise call client.Context(ctx, project) and use Engram data.
//
// The 15s context timeout matches the rest of the CLI. ctxParent is the
// parent context; we derive a timeout-bounded child so a hung Engram
// HTTP call cannot freeze the CLI.
func loadIdentitySource(ctxParent context.Context, cfg *config.Config, client engram.Client, project string) (*identitySource, error) {
	curatedObs, err := curated.LoadCurated(cfg.VaultPath, project)
	if err != nil {
		return nil, fmt.Errorf("load curated: %w", err)
	}
	if len(curatedObs) > 0 {
		return &identitySource{
			Observations: curatedObs,
			Origin:       "curated",
			CLILabel:     fmt.Sprintf("source: curated (%d note%s)", len(curatedObs), plural(len(curatedObs))),
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctxParent, 15*time.Second)
	defer cancel()
	obs, err := client.Context(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("engram context: %w", err)
	}
	return &identitySource{
		Observations: obs,
		Origin:       "raw/engram",
		CLILabel:     fmt.Sprintf("source: raw/engram fallback (%d observation%s)", len(obs), plural(len(obs))),
	}, nil
}
