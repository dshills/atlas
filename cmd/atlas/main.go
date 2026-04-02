package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dshills/atlas/internal/cli"
	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/db"
	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/goextractor"
	"github.com/dshills/atlas/internal/indexer"
	"github.com/dshills/atlas/internal/model"
	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/repo"
	"github.com/dshills/atlas/internal/store"
	"github.com/spf13/cobra"
)

const Version = "0.1.0"

var (
	flagRepo  string
	flagJSON  bool
	flagAgent bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "atlas",
		Short: "Atlas — structural and semantic index for source repositories",
	}

	rootCmd.PersistentFlags().StringVar(&flagRepo, "repo", "", "Explicit repository root path")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&flagAgent, "agent", false, "Output in compact agent JSON format")

	cliCtx := &cli.CLIContext{
		FlagRepo:  &flagRepo,
		FlagJSON:  &flagJSON,
		FlagAgent: &flagAgent,
	}

	rootCmd.AddCommand(
		versionCmd(),
		initCmd(),
		configCmd(),
		indexCmd(),
		reindexCmd(),
		cli.FindCmd(cliCtx),
		cli.WhoCallsCmd(cliCtx),
		cli.CallsCmd(cliCtx),
		cli.ImplementationsCmd(cliCtx),
		cli.ImportsCmd(cliCtx),
		cli.TestsForCmd(cliCtx),
		cli.TouchesCmd(cliCtx),
		cli.ListCmd(cliCtx),
		cli.StatsCmd(cliCtx),
		cli.StaleCmd(cliCtx),
		cli.SummarizeCmd(cliCtx, Version),
		cli.DoctorCmd(cliCtx),
		cli.ValidateCmd(cliCtx),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func outputMode() output.Mode {
	if flagAgent {
		return output.ModeAgent
	}
	if flagJSON {
		return output.ModeJSON
	}
	return output.ModeText
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Atlas version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := output.New(os.Stdout, outputMode())
			info := map[string]string{
				"version":        Version,
				"schema_version": fmt.Sprintf("%d", db.SchemaVersion),
				"go_version":     runtime.Version(),
			}
			if outputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Atlas version", Value: Version},
					{Key: "Schema version", Value: fmt.Sprintf("%d", db.SchemaVersion)},
					{Key: "Go version", Value: runtime.Version()},
				})
			}
			return f.Write(info)
		},
	}
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Atlas in the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := repo.FindRoot(flagRepo, "")
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			storageDir := filepath.Join(repoRoot, config.DefaultStorageDir)
			if err := os.MkdirAll(storageDir, 0o755); err != nil {
				return fmt.Errorf("creating storage directory: %w", err)
			}

			// Write default config if missing
			cfgPath := filepath.Join(storageDir, config.DefaultConfigFile)
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				if err := os.WriteFile(cfgPath, []byte(config.DefaultConfigYAML()), 0o644); err != nil {
					return fmt.Errorf("writing default config: %w", err)
				}
			}

			// Open and migrate DB
			dbPath := filepath.Join(storageDir, config.DefaultDBFile)
			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer func() { _ = database.Close() }()

			if err := db.Migrate(database); err != nil {
				return fmt.Errorf("running migrations: %w", err)
			}

			if err := db.InitMeta(database, Version); err != nil {
				return fmt.Errorf("initializing metadata: %w", err)
			}

			// Write manifest
			manifest := model.Manifest{
				RepoRoot:         repoRoot,
				SchemaVersion:    db.SchemaVersion,
				GeneratorVersion: Version,
			}
			manifestData, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling manifest: %w", err)
			}
			manifestPath := filepath.Join(storageDir, config.DefaultManifestFile)
			if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
				return fmt.Errorf("writing manifest: %w", err)
			}

			f := output.New(os.Stdout, outputMode())
			if outputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Initialized", Value: storageDir},
					{Key: "Database", Value: dbPath},
					{Key: "Config", Value: cfgPath},
					{Key: "Manifest", Value: manifestPath},
				})
			}
			return f.Write(map[string]string{
				"storage_dir": storageDir,
				"database":    dbPath,
				"config":      cfgPath,
				"manifest":    manifestPath,
			})
		},
	}
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print effective configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := repo.FindRoot(flagRepo, "")
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			cfg, err := config.LoadFromDir(repoRoot)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			f := output.New(os.Stdout, outputMode())
			if outputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Version", Value: fmt.Sprintf("%d", cfg.Version)},
					{Key: "Repo root", Value: cfg.RepoRoot},
					{Key: "Storage dir", Value: cfg.StorageDir},
					{Key: "Languages", Value: fmt.Sprintf("go=%v typescript=%v javascript=%v", cfg.Languages.Go, cfg.Languages.TypeScript, cfg.Languages.JavaScript)},
					{Key: "Max file size", Value: fmt.Sprintf("%d", cfg.Indexing.MaxFileSizeBytes)},
					{Key: "Summaries", Value: fmt.Sprintf("enabled=%v file=%v package=%v symbol=%v", cfg.Summaries.Enabled, cfg.Summaries.File, cfg.Summaries.Package, cfg.Summaries.Symbol)},
					{Key: "Output format", Value: cfg.Output.DefaultFormat},
				})
			}
			return f.Write(cfg)
		},
	}
}

func buildRegistry() *extractor.Registry {
	reg := extractor.NewRegistry()
	reg.Register(goextractor.New())
	return reg
}

func openDB(repoRoot string) (*store.Store, func(), error) {
	dbPath := filepath.Join(repoRoot, config.DefaultStorageDir, config.DefaultDBFile)
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.CheckSchemaVersion(database); err != nil {
		_ = database.Close()
		return nil, nil, err
	}
	s := store.New(database)
	cleanup := func() { _ = database.Close() }
	return s, cleanup, nil
}

func indexCmd() *cobra.Command {
	var flagSince string
	var flagStrict bool

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index the repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := repo.FindRoot(flagRepo, "")
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			cfg, err := config.LoadFromDir(repoRoot)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			s, cleanup, err := openDB(repoRoot)
			if err != nil {
				return err
			}
			defer cleanup()

			mode := "full"
			if flagSince != "" {
				mode = "incremental"
			}

			idx := indexer.New(repoRoot, cfg, s)
			idx.Registry = buildRegistry()
			idx.ModulePath = goextractor.DetectModulePath(repoRoot)
			idx.GeneratorVersion = Version
			result, err := idx.Run(mode, flagSince)
			if err != nil {
				return err
			}

			if flagStrict && idx.Diag.HasErrors() {
				return fmt.Errorf("strict mode: %d errors encountered", idx.Diag.ErrorCount())
			}

			f := output.New(os.Stdout, outputMode())
			if outputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Status", Value: result.Status},
					{Key: "Files scanned", Value: fmt.Sprintf("%d", result.FilesScanned)},
					{Key: "Files changed", Value: fmt.Sprintf("%d", result.FilesChanged)},
					{Key: "Files new", Value: fmt.Sprintf("%d", result.FilesNew)},
					{Key: "Files deleted", Value: fmt.Sprintf("%d", result.FilesDeleted)},
				})
			}
			return f.Write(result)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only index files changed since this Git revision")
	cmd.Flags().BoolVar(&flagStrict, "strict", false, "Fail on any error diagnostic")
	return cmd
}

func reindexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Reindex the repository from scratch",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := repo.FindRoot(flagRepo, "")
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			cfg, err := config.LoadFromDir(repoRoot)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			s, cleanup, err := openDB(repoRoot)
			if err != nil {
				return err
			}
			defer cleanup()

			idx := indexer.New(repoRoot, cfg, s)
			idx.Registry = buildRegistry()
			idx.ModulePath = goextractor.DetectModulePath(repoRoot)
			idx.GeneratorVersion = Version
			if err := idx.ClearAll(); err != nil {
				return fmt.Errorf("clearing data: %w", err)
			}

			result, err := idx.Run("full", "")
			if err != nil {
				return err
			}

			f := output.New(os.Stdout, outputMode())
			if outputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Status", Value: result.Status},
					{Key: "Files scanned", Value: fmt.Sprintf("%d", result.FilesScanned)},
					{Key: "Files changed", Value: fmt.Sprintf("%d", result.FilesChanged)},
				})
			}
			return f.Write(result)
		},
	}
}
