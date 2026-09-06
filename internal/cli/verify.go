package cli

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/integration"
	"github.com/webxsid/pdg/internal/reconcile"
	"github.com/webxsid/pdg/internal/state"
)

var verifyLive bool

const liveVerificationTimeout = 30 * time.Second

var verifyCmd = &cobra.Command{Use: "verify [integration]", Short: "Verify local integration artifacts", Args: cobra.MaximumNArgs(1), RunE: runVerify}
var repairCmd = &cobra.Command{Use: "repair [integration]", Short: "Repair local integration artifacts", Args: cobra.MaximumNArgs(1), RunE: runRepair}

func loadProject() (reconcile.Project, error) {
	cfg, err := config.LoadConfig(config.DefaultFilename)
	if err != nil {
		return reconcile.Project{}, fmt.Errorf("load project config: %w", err)
	}
	projectState, err := state.Load(filepath.Join(".pdg", "state.json"))
	if err != nil {
		return reconcile.Project{}, fmt.Errorf("load project state: %w", err)
	}
	return reconcile.Project{Config: *cfg, State: projectState, Root: "."}, nil
}

func runVerify(cmd *cobra.Command, args []string) error {
	if verifyLive {
		fmt.Println("Loading project configuration and state...")
		project, err := loadProject()
		if err != nil {
			return err
		}
		name, err := selectVerificationIntegration(args, project.Config)
		if err != nil {
			return err
		}
		if name != integration.StandardSite {
			return fmt.Errorf("live verification for %s is not implemented yet", name)
		}
		fmt.Println("Verifying deployed Standard.site...")
		findings := reconcile.VerifyStandardSiteLiveWithProgress(cmd.Context(), project, &http.Client{Timeout: liveVerificationTimeout}, func(resource, rawURL string) {
			fmt.Printf("  Checking %s (%s)\n", resource, rawURL)
		})
		printFindings(findings)
		for _, finding := range findings {
			if finding.Status != reconcile.OK {
				return fmt.Errorf("live verification failed")
			}
		}
		fmt.Println("Live Standard.site verification passed.")
		return nil
	}
	project, err := loadProject()
	if err != nil {
		return err
	}
	name, err := selectVerificationIntegration(args, project.Config)
	if err != nil {
		return err
	}
	findings := reconcile.VerifyStandardSite(project)
	if name != integration.StandardSite {
		findings = nil
	}
	printFindings(findings)
	for _, finding := range findings {
		if finding.Status != reconcile.OK {
			return fmt.Errorf("project verification failed")
		}
	}
	fmt.Println("Project verification passed.")
	return nil
}

func runRepair(cmd *cobra.Command, args []string) error {
	project, err := loadProject()
	if err != nil {
		return err
	}
	name, err := selectVerificationIntegration(args, project.Config)
	if err != nil {
		return err
	}
	findings := reconcile.VerifyStandardSite(project)
	if name != integration.StandardSite {
		findings = nil
	}
	unsafe := false
	repaired := false
	for _, finding := range findings {
		if finding.Status != reconcile.OK && !finding.Repairable {
			unsafe = true
		}
	}
	if unsafe {
		printFindings(findings)
		return fmt.Errorf("automatic repair is unsafe; no changes made")
	}
	if err := reconcile.RepairStandardSite(project, findings); err != nil {
		return fmt.Errorf("repair Standard.site: %w", err)
	}
	for _, finding := range findings {
		if finding.Repairable {
			repaired = true
		}
	}
	if repaired {
		fmt.Println("✓ Restored Standard.site local artifacts.")
	}
	updated := reconcile.VerifyStandardSite(project)
	printFindings(updated)
	for _, finding := range updated {
		if finding.Status != reconcile.OK {
			return fmt.Errorf("repair incomplete")
		}
	}
	if !repaired {
		fmt.Println("No repairs required.")
	}
	return nil
}

func selectVerificationIntegration(args []string, cfg config.Config) (integration.Integration, error) {
	if len(args) == 0 {
		if cfg.Integrations.StandardSite.Enabled {
			return integration.StandardSite, nil
		}
		if cfg.Integrations.Bluesky.Enabled {
			return integration.Bluesky, nil
		}
		return "", fmt.Errorf("no integrations are configured; run `pdg add <integration>` first")
	}
	name := args[0]
	if len(args) == 1 {
		name = args[0]
	}
	d, err := integration.Resolve(name)
	if err != nil {
		return "", err
	}
	if !isConfigured(d.Name, cfg) {
		return "", fmt.Errorf("%s is not configured for this project; run `pdg add %s` first", integrationLabel(d.Name), d.Name)
	}
	return d.Name, nil
}

func printFindings(findings []reconcile.Finding) {
	for _, f := range findings {
		mark := "✓"
		if f.Status != reconcile.OK {
			mark = "✗"
		}
		fmt.Printf("%s %s\n  %s\n", mark, f.Resource, f.Message)
	}
}

func init() {
	verifyCmd.Flags().BoolVar(&verifyLive, "live", false, "verify the deployed site (not implemented yet)")
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(repairCmd)
}
