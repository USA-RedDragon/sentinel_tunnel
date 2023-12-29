package sentinel

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type TunnellingConfiguration struct {
	SentinelsAddressesList []string
	Password               string
	Databases              []TunnellingDbConfig
}

type TunnellingDbConfig struct {
	Name      string
	LocalPort string
}

func (t TunnellingDbConfig) String() string {
	return t.Name + ":" + t.LocalPort
}

func (t *TunnellingDbConfig) Set(s string) error {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return fmt.Errorf("failed to split host port: %w", err)
	}

	t.Name = host
	t.LocalPort = port
	return nil
}

func (t TunnellingDbConfig) Type() string {
	return "string"
}

//nolint:golint,gochecknoglobals
var (
	ConfigFileKey = "config"
	SentinelsKey  = "sentinels"
	PasswordKey   = "password"
	DatabasesKey  = "databases"
)

func RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(ConfigFileKey, "c", "", "Config file path")
	cmd.Flags().StringSliceP(SentinelsKey, "s", []string{}, "Comma-separated list of Sentinel addresses")
	cmd.Flags().StringP(PasswordKey, "p", "", "Sentinel password")
	cmd.Flags().StringSliceP(DatabasesKey, "d", []string{}, "Comma-separated list of Databases to expose")
}

func LoadConfig(cmd *cobra.Command) (TunnellingConfiguration, error) {
	var config TunnellingConfiguration

	// Load flags from envs
	ctx, cancel := context.WithCancelCause(cmd.Context())
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if ctx.Err() != nil {
			return
		}
		optName := strings.ToUpper(f.Name)
		optName = strings.ReplaceAll(optName, "-", "_")
		varName := "ST_" + optName
		if val, ok := os.LookupEnv(varName); !f.Changed && ok {
			if err := f.Value.Set(val); err != nil {
				cancel(err)
			}
			f.Changed = true
		}
	})
	if ctx.Err() != nil {
		return config, fmt.Errorf("failed to load env: %w", context.Cause(ctx))
	}

	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		panic(err)
	}
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return config, fmt.Errorf("failed to read config: %w", err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			return config, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	if cmd.Flags().Changed(SentinelsKey) {
		config.SentinelsAddressesList, err = cmd.Flags().GetStringSlice(SentinelsKey)
		if err != nil {
			panic(err)
		}
	}

	if cmd.Flags().Changed(PasswordKey) {
		config.Password, err = cmd.Flags().GetString(PasswordKey)
		if err != nil {
			panic(err)
		}
	}

	if cmd.Flags().Changed(DatabasesKey) {
		databases, err := cmd.Flags().GetStringSlice(DatabasesKey)
		if err != nil {
			panic(err)
		}
		config.Databases = make([]TunnellingDbConfig, 0, len(databases))

		for _, raw := range databases {
			var db TunnellingDbConfig
			if err := db.Set(raw); err != nil {
				return config, fmt.Errorf("failed to parse database %s: %w", db, err)
			}
			config.Databases = append(config.Databases, db)
		}
	}

	return config, nil
}
