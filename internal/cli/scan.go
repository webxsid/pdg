package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/scan"
)

var scnCmd = &cobra.Command{
	Use:   "scan [directory]",
	Short: "Scan an existing website directory for publishable content",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var dir string
		if len(args) == 1 {
			dir = args[0]
		}

		return runScan(cmd.Context(), dir)
	},
}

func runScan(ctx context.Context, dir string) error {
	if dir == "" {
		cfg, err := config.LoadConfig(config.DefaultFilename)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		dir = cfg.Site.OutputDir
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	filesystem := os.DirFS(dir)

	scanner := scan.NewScanner()

	result, err := scanner.Scan(ctx, filesystem)
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	printScanResult(result)

	return nil
}

func printScanResult(result scan.Result) {
	fmt.Println("Scan Result:")
	fmt.Printf("Documents found: %d\n", len(result.Documents))
	for _, doc := range result.Documents {
		fmt.Printf("- %s (Title: %s)\n URL:    %s\n", doc.Path, doc.Title, doc.CanonicalURL)
	}

	if len(result.Skipped) == 0 {
		return
	}

	fmt.Printf("\nSkipped files: %d\n", len(result.Skipped))
	for _, skipped := range result.Skipped {
		fmt.Printf("- %s (Reason: %s)\n", skipped.Path, skipped.Reason)
	}
}
