package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/ahlert/telvar/assets"
	"github.com/ahlert/telvar/internal/config"
	ghconnector "github.com/ahlert/telvar/internal/connector/github"
	"github.com/ahlert/telvar/internal/scheduler"
	"github.com/ahlert/telvar/internal/scorecard"
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
	root.AddCommand(statusCmd())
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

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			var wg sync.WaitGroup

			if cfg.Connectors.GitHub != nil {
				interval, parseErr := time.ParseDuration(cfg.Discovery.ScanInterval)
				if parseErr != nil {
					return fmt.Errorf("parsing scan_interval: %w", parseErr)
				}

				client := ghconnector.NewClient(cfg.Connectors.GitHub)
				scanner := ghconnector.NewScanner(client, db, &cfg.Discovery, scorecard.NewRunner(cfg.Scorecards))
				sched := scheduler.New(scanner, interval)

				wg.Add(1)
				go func() {
					defer wg.Done()
					sched.Start(ctx)
				}()
			}

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
			serveErr := srv.ListenAndServe(ctx, addr)
			stop()
			wg.Wait()
			return serveErr
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
			scanner := ghconnector.NewScanner(client, db, &cfg.Discovery, scorecard.NewRunner(cfg.Scorecards))

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

func statusCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show catalog and discovery status",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.New(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			count, err := db.CountEntities()
			if err != nil {
				return fmt.Errorf("counting entities: %w", err)
			}

			fmt.Printf("Entities: %d\n", count)

			run, err := db.LatestDiscoveryRun()
			if err != nil {
				return fmt.Errorf("fetching latest run: %w", err)
			}

			if run == nil {
				fmt.Println("Last discovery: never")
				return nil
			}

			fmt.Printf("Last discovery:\n")
			fmt.Printf("  Source:   %s\n", run.Source)
			fmt.Printf("  Status:   %s\n", run.Status)
			fmt.Printf("  Started:  %s\n", run.StartedAt.Format("2006-01-02 15:04:05 UTC"))
			if run.FinishedAt != nil {
				fmt.Printf("  Finished: %s\n", run.FinishedAt.Format("2006-01-02 15:04:05 UTC"))
				fmt.Printf("  Duration: %s\n", run.FinishedAt.Sub(run.StartedAt).Round(time.Millisecond))
			}
			fmt.Printf("  Entities: %d\n", run.EntitiesFound)

			return nil
		},
	}

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
