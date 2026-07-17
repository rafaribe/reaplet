package talos

import (
	"context"
	"fmt"
	"os"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
)

// NewClient creates a Talos machinery client from the talosconfig.
// It reads the config from the path specified, or falls back to the default location.
func NewClient(ctx context.Context, configPath string) (*client.Client, error) {
	if configPath == "" {
		configPath = defaultConfigPath()
	}

	cfg, err := config.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("opening talosconfig %s: %w", configPath, err)
	}

	opts := []client.OptionFunc{
		client.WithConfig(cfg),
	}

	c, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating talos client: %w", err)
	}

	return c, nil
}

func defaultConfigPath() string {
	if p := os.Getenv("TALOSCONFIG"); p != "" {
		return p
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "/var/run/secrets/talos.dev/config"
	}

	return home + "/.talos/config"
}
