package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/daemon"
)

// version is set via ldflags at build time (-X main.version=...).
var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "mesh-daemon",
		Short: "Mesh daemon — portable agent-body runtime",
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Mesh daemon with REST API",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			d, err := daemon.New(cfg)
			if err != nil {
				return fmt.Errorf("failed to create daemon: %w", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			return d.Start(ctx)
		},
	}
	serveCmd.Flags().String("config", "/etc/mesh/config.yaml", "Path to config file")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the Mesh daemon version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
