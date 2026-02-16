package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/wesm/msgvault/internal/notion"
	"github.com/wesm/msgvault/internal/store"
)

var (
	notionToken    string
	notionLimit    int
	notionNoResume bool
)

var notionAddCmd = &cobra.Command{
	Use:   "notion-add <workspace-id>",
	Short: "Add a Notion workspace for archival",
	Long: `Store a Notion integration token for syncing workspace pages.

Requires a Notion internal integration token (starts with ntn_).
Create one at https://www.notion.so/my-integrations

Examples:
  msgvault notion-add myworkspace --token ntn_...
  msgvault notion-add synnada --token "$NOTION_INTEGRATION_TOKEN"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceID := args[0]

		if notionToken == "" {
			return fmt.Errorf("--token is required (Notion integration token starting with ntn_)")
		}

		// Initialize database
		dbPath := cfg.DatabaseDSN()
		s, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer s.Close()

		if err := s.InitSchema(); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}

		// Store the token via Notion OAuth manager
		mgr := notion.NewManager(cfg.TokensDir(), logger)
		if err := mgr.StoreToken(cmd.Context(), workspaceID, notionToken); err != nil {
			return fmt.Errorf("store token: %w", err)
		}

		// Create source record in database
		if _, err := s.GetOrCreateSource("notion", workspaceID); err != nil {
			return fmt.Errorf("create source: %w", err)
		}

		fmt.Printf("Notion workspace %q added successfully!\n", workspaceID)
		fmt.Printf("You can now run: msgvault notion-sync %s\n", workspaceID)

		return nil
	},
}

var notionSyncCmd = &cobra.Command{
	Use:   "notion-sync [workspace-id]",
	Short: "Full sync of Notion workspace pages",
	Long: `Perform a full synchronization of a Notion workspace.

Downloads all pages, converts them to markdown, and stores them locally.
Supports resumption from interruption - just run again to continue.

If no workspace-id is specified, syncs all configured Notion workspaces.

Examples:
  msgvault notion-sync myworkspace
  msgvault notion-sync myworkspace --limit 10
  msgvault notion-sync                          # Sync all workspaces`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if notionLimit < 0 {
			return fmt.Errorf("--limit must be a non-negative number")
		}

		// Open database
		dbPath := cfg.DatabaseDSN()
		s, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer s.Close()

		if err := s.InitSchema(); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}

		// Determine which workspaces to sync
		workspaces, err := resolveNotionWorkspaces(s, args)
		if err != nil {
			return err
		}

		// Set up context with cancellation
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nInterrupted. Saving checkpoint...")
			cancel()
		}()

		var syncErrors []string
		for _, ws := range workspaces {
			if ctx.Err() != nil {
				break
			}

			if err := runNotionFullSync(ctx, s, ws); err != nil {
				syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", ws, err))
				continue
			}
		}

		if len(syncErrors) > 0 {
			fmt.Println()
			fmt.Println("Errors:")
			for _, e := range syncErrors {
				fmt.Printf("  %s\n", e)
			}
			return fmt.Errorf("%d workspace(s) failed to sync", len(syncErrors))
		}

		return nil
	},
}

var notionSyncIncrementalCmd = &cobra.Command{
	Use:   "notion-sync-incremental [workspace-id]",
	Short: "Sync only changed Notion pages",
	Long: `Perform an incremental sync of a Notion workspace.

Only fetches pages that have been modified since the last sync.
Requires a prior full sync to establish a baseline.

If no workspace-id is specified, syncs all configured Notion workspaces.

Examples:
  msgvault notion-sync-incremental myworkspace
  msgvault notion-sync-incremental               # Sync all workspaces`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Open database
		dbPath := cfg.DatabaseDSN()
		s, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer s.Close()

		if err := s.InitSchema(); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}

		// Determine which workspaces to sync
		workspaces, err := resolveNotionWorkspaces(s, args)
		if err != nil {
			return err
		}

		// Set up context with cancellation
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nInterrupted. Saving checkpoint...")
			cancel()
		}()

		var syncErrors []string
		for _, ws := range workspaces {
			if ctx.Err() != nil {
				break
			}

			if err := runNotionIncrementalSync(ctx, s, ws); err != nil {
				syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", ws, err))
				continue
			}
		}

		if len(syncErrors) > 0 {
			fmt.Println()
			fmt.Println("Errors:")
			for _, e := range syncErrors {
				fmt.Printf("  %s\n", e)
			}
			return fmt.Errorf("%d workspace(s) failed to sync", len(syncErrors))
		}

		return nil
	},
}

// resolveNotionWorkspaces determines which workspace IDs to sync.
func resolveNotionWorkspaces(s *store.Store, args []string) ([]string, error) {
	if len(args) == 1 {
		return []string{args[0]}, nil
	}

	sources, err := s.ListSources("notion")
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no Notion workspaces configured - run 'notion-add' first")
	}

	var workspaces []string
	mgr := notion.NewManager(cfg.TokensDir(), logger)
	for _, src := range sources {
		if !mgr.HasToken(src.Identifier) {
			fmt.Printf("Skipping %s (no token - run 'notion-add' first)\n", src.Identifier)
			continue
		}
		workspaces = append(workspaces, src.Identifier)
	}

	if len(workspaces) == 0 {
		return nil, fmt.Errorf("no workspaces have valid tokens - run 'notion-add' first")
	}

	return workspaces, nil
}

func runNotionFullSync(ctx context.Context, s *store.Store, workspaceID string) error {
	// Load token
	mgr := notion.NewManager(cfg.TokensDir(), logger)
	token, err := mgr.LoadToken(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load token: %w (run 'notion-add' first)", err)
	}

	// Create Notion client
	rl := notion.NewRateLimiter(notion.DefaultTokensPerSec)
	client := notion.NewClient(token,
		notion.WithLogger(logger),
		notion.WithRateLimiter(rl),
	)
	defer client.Close()

	// Create store adapter
	nstore := notion.NewNotionStore(s, logger)

	// Set up sync options
	opts := notion.DefaultSyncOptions()
	opts.Limit = notionLimit
	opts.NoResume = notionNoResume

	// Create syncer with progress reporter
	syncer := notion.NewSyncer(client, nstore, opts).
		WithLogger(logger).
		WithProgress(&NotionCLIProgress{})

	// Run sync
	startTime := time.Now()
	fmt.Printf("Starting full sync for Notion workspace %q\n\n", workspaceID)

	result, err := syncer.Full(ctx, workspaceID)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("\nSync interrupted. Run again to resume.")
			return nil
		}
		return fmt.Errorf("sync failed: %w", err)
	}

	// Print summary
	fmt.Println()
	fmt.Println("Sync complete!")
	fmt.Printf("  Duration:  %s\n", result.Duration.Round(time.Second))
	fmt.Printf("  Pages:     %d processed, %d added, %d skipped\n",
		result.PagesProcessed, result.PagesAdded, result.PagesSkipped)
	if result.Errors > 0 {
		fmt.Printf("  Errors:    %d\n", result.Errors)
	}

	if result.PagesAdded > 0 && result.Duration.Seconds() >= 1 {
		rate := float64(result.PagesAdded) / result.Duration.Seconds()
		fmt.Printf("  Rate:      %.1f pages/sec\n", rate)
	}

	elapsed := time.Since(startTime)
	logger.Info("notion sync completed",
		"workspace", workspaceID,
		"pages_added", result.PagesAdded,
		"elapsed", elapsed,
	)

	return nil
}

func runNotionIncrementalSync(ctx context.Context, s *store.Store, workspaceID string) error {
	// Load token
	mgr := notion.NewManager(cfg.TokensDir(), logger)
	token, err := mgr.LoadToken(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load token: %w (run 'notion-add' first)", err)
	}

	// Create Notion client
	rl := notion.NewRateLimiter(notion.DefaultTokensPerSec)
	client := notion.NewClient(token,
		notion.WithLogger(logger),
		notion.WithRateLimiter(rl),
	)
	defer client.Close()

	// Create store adapter
	nstore := notion.NewNotionStore(s, logger)

	// Set up sync options
	opts := notion.DefaultSyncOptions()

	// Create syncer with progress reporter
	syncer := notion.NewSyncer(client, nstore, opts).
		WithLogger(logger).
		WithProgress(&NotionCLIProgress{})

	// Run incremental sync
	startTime := time.Now()
	fmt.Printf("Starting incremental sync for Notion workspace %q\n\n", workspaceID)

	result, err := syncer.Incremental(ctx, workspaceID)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("\nSync interrupted. Run again to resume.")
			return nil
		}
		return fmt.Errorf("sync failed: %w", err)
	}

	// Print summary
	fmt.Println()
	fmt.Println("Sync complete!")
	fmt.Printf("  Duration:  %s\n", result.Duration.Round(time.Second))
	fmt.Printf("  Pages:     %d processed, %d updated\n",
		result.PagesProcessed, result.PagesUpdated)
	if result.Errors > 0 {
		fmt.Printf("  Errors:    %d\n", result.Errors)
	}

	elapsed := time.Since(startTime)
	logger.Info("notion incremental sync completed",
		"workspace", workspaceID,
		"pages_updated", result.PagesUpdated,
		"elapsed", elapsed,
	)

	return nil
}

// NotionCLIProgress implements notion.SyncProgress for terminal output.
type NotionCLIProgress struct {
	startTime time.Time
	lastPrint time.Time
	processed int
	added     int
}

func (p *NotionCLIProgress) OnPageStart(pageID, title string) {
	if p.startTime.IsZero() {
		p.startTime = time.Now()
		p.lastPrint = p.startTime
	}
}

func (p *NotionCLIProgress) OnPageComplete(pageID string) {
	p.added++
	p.printProgress()
}

func (p *NotionCLIProgress) OnProgress(processed, total int) {
	p.processed = processed
	p.printProgress()
}

func (p *NotionCLIProgress) OnError(pageID string, err error) {
	fmt.Printf("\n  Error syncing %s: %v\n", pageID, err)
}

func (p *NotionCLIProgress) printProgress() {
	if time.Since(p.lastPrint) < 2*time.Second {
		return
	}
	p.lastPrint = time.Now()

	elapsed := time.Since(p.startTime)
	rate := 0.0
	if elapsed.Seconds() >= 1 {
		rate = float64(p.processed) / elapsed.Seconds()
	}

	elapsedStr := formatDuration(elapsed)
	fmt.Printf("\r  Processed: %d | Rate: %.1f/s | Elapsed: %s    ",
		p.processed, rate, elapsedStr)
}

func init() {
	notionAddCmd.Flags().StringVar(&notionToken, "token", "", "Notion integration token (required, starts with ntn_)")
	_ = notionAddCmd.MarkFlagRequired("token")
	rootCmd.AddCommand(notionAddCmd)

	notionSyncCmd.Flags().IntVar(&notionLimit, "limit", 0, "Limit number of pages (for testing)")
	notionSyncCmd.Flags().BoolVar(&notionNoResume, "noresume", false, "Force fresh sync (don't resume)")
	rootCmd.AddCommand(notionSyncCmd)

	rootCmd.AddCommand(notionSyncIncrementalCmd)
}
