package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/scan"
	"github.com/webxsid/pdg/internal/state"
)

var scnCmd = &cobra.Command{
	Use:   "scan [directory]",
	Short: "Scan an existing website directory for publishable content",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := ""
		if len(args) == 1 {
			dir = args[0]
		}
		return runScan(cmd.Context(), dir)
	},
}

func runScan(ctx context.Context, dir string) error {
	cfg, err := config.LoadConfig(config.DefaultFilename)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if dir == "" {
		dir = cfg.Integrations.StandardSite.PublicDir
	}
	if dir == "" {
		return fmt.Errorf("scan directory is not configured; pass a directory or set integrations.standard_site.public_dir")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("failed to stat directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	previous, err := state.Load(filepath.Join(".pdg", "state.json"))
	if err != nil {
		return err
	}
	result, scanErr := scan.NewScanner().ScanStateful(ctx, os.DirFS(dir), scan.StatefulOptions{Config: *cfg, Previous: previous})
	fmt.Printf("Scanning %s\n", dir)
	printStatefulScanResult(result)
	if scanErr != nil {
		return scanErr
	}
	return state.Write(filepath.Join(".pdg", "state.json"), result.State)
}

func printStatefulScanResult(result scan.StatefulResult) {
	fmt.Printf("Scanned publication candidates: %d\n", len(result.Documents))
	for _, document := range result.Documents {
		fmt.Printf("- %s [%s] targets: %s\n", document.Path, document.Status, strings.Join(selectedTargets(document), ", "))
	}
	for _, err := range result.Errors {
		fmt.Printf("! %s\n", err)
	}
}

func selectedTargets(document *state.DocumentState) []string {
	result := []string{}
	for target, value := range document.Targets {
		if value.Selected {
			result = append(result, string(target))
		}
	}
	sort.Strings(result)
	return result
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
