package cmd

import (
	"flag"
	"fmt"
	"rsshub/cli/control"
	"rsshub/internal/config"
)

func SetWorkers(args []string) error {
	fs := flag.NewFlagSet("set-workers", flag.ContinueOnError)
	count := fs.Int("count", 0, "number of workers")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *count <= 0 || *count > 15 {
		return fmt.Errorf("number of workers should be between 1 and 15")
	}

	c := control.NewClient(config.Load().ControlAddr)
	old, err := c.SetWorkers(*count)
	if err != nil {
		return fmt.Errorf("could not set workers: %w", err)
	}
	fmt.Printf("Number of workers changed from %d to %d\n", old, *count)
	return nil
}
