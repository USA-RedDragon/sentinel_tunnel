package cmd

import (
	"context"
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

	cmd.Flags().StringP("config", "c", "", "Config file path")

	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	slog.Info("Redis Sentinel Tunnel",
		"version", cmd.Annotations["version"], "commit", cmd.Annotations["commit"], "date", cmd.Annotations["date"])

	var configFile string
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		panic(err)
	}
	if configFile == "" {
		if len(args) != 0 {
			configFile = args[0]
		} else {
			return ErrMissingConfig
		}
	}

	stClient, err := sentinel.NewTunnellingClient(configFile)
	if err != nil {
		return fmt.Errorf("failed to create tunnelling client: %w", err)
	}

	if err := stClient.ListenAndServe(context.Background()); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
