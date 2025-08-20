package cmd

import (
	"flag"
	"fmt"
	"time"

	"rsshub/cli/control"
	"rsshub/internal/config"
)

func SetInterval(args []string) error {
	fs := flag.NewFlagSet("set-interval", flag.ContinueOnError)
	duration := fs.String("duration", "", "fetch interval duration (e.g., 2m)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *duration == "" {
		return fmt.Errorf("usage: rsshub set-interval --duration 2m")
	}

	d, err := time.ParseDuration(*duration)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	c := control.NewClient(config.Load().ControlAddr)
	old, err := c.SetInterval(d)
	if err != nil {
		return fmt.Errorf("could not set interval: %w", err)
	}
	fmt.Printf("Interval of fetching feeds changed from %s to %s\n", old.String(), d.String())
	return nil
}
