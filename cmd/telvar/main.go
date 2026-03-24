package main

import (
	"fmt"
	"os"

	"github.com/ahlert/telvar/internal/config"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "telvar",
		Short:   "Zero-authoring developer portal",
		Long:    "Telvar auto-discovers your services from GitHub and Kubernetes.\nNo YAML to write. No plugins to maintain. One binary.",
		Version: version,
	}

	root.AddCommand(serveCmd())
	root.AddCommand(discoverCmd())
	root.AddCommand(configCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	var cfgPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Telvar web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			fmt.Printf("Telvar %s starting on :%d\n", version, cfg.Server.Port)
			// TODO: wire up server
			return nil
		},
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", "telvar.yaml", "path to config file")
	return cmd
}

func discoverCmd() *cobra.Command {
	var cfgPath string

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Run discovery against configured sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			_ = cfg
			fmt.Println("Running discovery...")
			// TODO: run discovery pipeline
			return nil
		},
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", "telvar.yaml", "path to config file")
	return cmd
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
	}

	var output string
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a default config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return config.WriteDefault(output)
		},
	}
	initCmd.Flags().StringVarP(&output, "output", "o", "telvar.yaml", "output path for config file")

	cmd.AddCommand(initCmd)
	return cmd
}
