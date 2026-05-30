package main

import (
	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/schedule"
	"github.com/nicolasperalta/silo2/internal/setup"
)

func runSetupRoutine(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Ensure productive hours defaults exist in config for first-time use.
	if len(cfg.ProductiveHours) == 0 {
		cfg.ProductiveHours = config.DefaultProductiveHours()
	}

	schedulePath := cfg.SchedulePath
	if schedulePath == "" {
		schedulePath = config.DefaultSchedulePath()
	}
	store := schedule.NewStore(schedulePath)

	return setup.RunInterview(cfg, store)
}
