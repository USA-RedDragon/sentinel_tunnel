package cmd

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/USA-RedDragon/sentinel_tunnel/internal/sentinel"
	"github.com/spf13/cobra"
)

var (
	ErrMissingConfig = errors.New("missing configuration")
)

func NewCommand(version, commit, date string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sentinel_tunnel",
		Version: fmt.Sprintf("%s (%s built %s)", version, commit, date),
		Annotations: map[string]string{
			"version": version,
			"commit":  commit,
			"date":    date,
		},
		RunE:          run,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	sentinel.RegisterFlags(cmd)
	return cmd
}

func run(cmd *cobra.Command, _ []string) error {
	slog.Info("Redis Sentinel Tunnel",
		"version", cmd.Annotations["version"], "commit", cmd.Annotations["commit"], "date", cmd.Annotations["date"])

	config, err := sentinel.LoadConfig(cmd)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	stClient, err := sentinel.NewTunnellingClient(config)
	if err != nil {
		return fmt.Errorf("failed to create tunnelling client: %w", err)
	}

	if err := stClient.ListenAndServe(cmd.Context()); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
