package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/ahlert/telvar/assets"
	"github.com/ahlert/telvar/internal/config"
	ghconnector "github.com/ahlert/telvar/internal/connector/github"
	"github.com/ahlert/telvar/internal/store"
	"github.com/ahlert/telvar/internal/web"
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
	var (
		cfgPath string
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Telvar web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			db, err := store.New(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			tmplFS, err := fs.Sub(assets.Templates, "templates")
			if err != nil {
				return fmt.Errorf("accessing embedded templates: %w", err)
			}
			statFS, err := fs.Sub(assets.Static, "static")
			if err != nil {
				return fmt.Errorf("accessing embedded static files: %w", err)
			}
			srv, err := web.New(db, tmplFS, statFS)
			if err != nil {
				return fmt.Errorf("creating web server: %w", err)
			}

			addr := fmt.Sprintf(":%d", cfg.Server.Port)
			slog.Info("Telvar starting", "version", version, "addr", addr)
			return srv.ListenAndServe(addr)
		},
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", "telvar.yaml", "path to config file")
	cmd.Flags().StringVarP(&dbPath, "db", "d", "telvar.db", "path to SQLite database")
	return cmd
}

func discoverCmd() *cobra.Command {
	var (
		cfgPath string
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Run discovery against configured sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			db, err := store.New(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if cfg.Connectors.GitHub == nil {
				return fmt.Errorf("no GitHub connector configured")
			}

			client := ghconnector.NewClient(cfg.Connectors.GitHub)
			scanner := ghconnector.NewScanner(client, db, &cfg.Discovery)

			slog.Info("starting discovery", "source", "github", "org", cfg.Connectors.GitHub.Org)
			if err := scanner.Run(cmd.Context()); err != nil {
				return fmt.Errorf("discovery failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", "telvar.yaml", "path to config file")
	cmd.Flags().StringVarP(&dbPath, "db", "d", "telvar.db", "path to SQLite database")
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
