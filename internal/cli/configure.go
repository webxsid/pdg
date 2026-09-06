package cli

import (
	"fmt"
	"os"
	"reflect"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/webxsid/pdg/internal/config"
	"github.com/webxsid/pdg/internal/integration"
)

var configureCmd = &cobra.Command{
	Use:   "configure [integration]",
	Short: "Configure an existing project integration",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigure(args)
	},
}

func runConfigure(args []string) error {
	cfg, err := config.LoadConfig(config.DefaultFilename)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	name, err := configuredIntegration(args, *cfg)
	if err != nil {
		return err
	}
	updated := *cfg
	switch name {
	case integration.StandardSite:
		paths, err := configurePublicationPaths("Standard.site", updated.Integrations.StandardSite.Paths)
		if err != nil {
			return err
		}
		updated.Integrations.StandardSite.Paths = paths
	case integration.Bluesky:
		fmt.Println("Bluesky has no project publication paths; posting is explicit via `pdg post`.")
		return nil
	}
	if reflect.DeepEqual(*cfg, updated) {
		fmt.Println("No configuration changes made.")
		return nil
	}
	if err := config.WriteConfig(config.DefaultFilename, updated); err != nil {
		return fmt.Errorf("save integration configuration: %w", err)
	}
	fmt.Printf("✓ %s configuration saved to %s\n", integrationLabel(name), config.DefaultFilename)
	return nil
}

func configuredIntegration(args []string, cfg config.Config) (integration.Integration, error) {
	if len(args) == 1 {
		definition, err := integration.Resolve(args[0])
		if err != nil {
			return "", err
		}
		if !isConfigured(definition.Name, cfg) {
			return "", fmt.Errorf("%s is not configured for this project; run `pdg add %s` first", integrationLabel(definition.Name), definition.Name)
		}
		return definition.Name, nil
	}
	options := make([]string, 0, 2)
	names := make([]integration.Integration, 0, 2)
	for _, definition := range []integration.Definition{{Name: integration.StandardSite}, {Name: integration.Bluesky}} {
		if isConfigured(definition.Name, cfg) {
			options = append(options, integrationLabel(definition.Name))
			names = append(names, definition.Name)
		}
	}
	if len(options) == 0 {
		return "", fmt.Errorf("no integrations are configured; run `pdg add <integration>` first")
	}
	if len(options) == 1 {
		return names[0], nil
	}
	var selected string
	if err := survey.AskOne(&survey.Select{Message: "Configure an integration:", Options: options}, &selected, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
		return "", err
	}
	for i, option := range options {
		if option == selected {
			return names[i], nil
		}
	}
	return "", fmt.Errorf("selected integration is unavailable")
}

func isConfigured(name integration.Integration, cfg config.Config) bool {
	switch name {
	case integration.StandardSite:
		return cfg.Integrations.StandardSite.Enabled
	case integration.Bluesky:
		return cfg.Integrations.Bluesky.Enabled
	default:
		return false
	}
}

func configurePublicationPaths(name string, current []config.PublicationPath) ([]config.PublicationPath, error) {
	paths, err := config.NormalizePublicationPaths(current)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Configure %s\nPublication paths:\n", name)
	printPublicationPaths(paths)
	for {
		var action string
		if err := survey.AskOne(&survey.Select{Message: "What would you like to do?", Options: []string{"Add paths", "Edit path", "Remove paths", "Done"}, Default: "Done"}, &action, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
			return nil, err
		}
		switch action {
		case "Add paths":
			added, err := promptPublicationPaths(name)
			if err != nil {
				return nil, err
			}
			paths = append(paths, added...)
			paths, err = config.NormalizePublicationPaths(paths)
			if err != nil {
				return nil, err
			}
		case "Edit path":
			if len(paths) == 0 {
				fmt.Println("No publication paths configured.")
				continue
			}
			if err := editPublicationPath(paths); err != nil {
				return nil, err
			}
		case "Remove paths":
			if len(paths) == 0 {
				fmt.Println("No publication paths configured.")
				continue
			}
			paths, err = removePublicationPaths(paths)
			if err != nil {
				return nil, err
			}
		case "Done":
			return paths, nil
		}
		fmt.Println("Publication paths:")
		printPublicationPaths(paths)
	}
}

func printPublicationPaths(paths []config.PublicationPath) {
	if len(paths) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, item := range paths {
		fmt.Printf("  %-20s %s\n", item.Path, item.Publish)
	}
}

func editPublicationPath(paths []config.PublicationPath) error {
	options := make([]string, len(paths))
	for i, item := range paths {
		options[i] = item.Path
	}
	var selected string
	if err := survey.AskOne(&survey.Select{Message: "Select path:", Options: options}, &selected, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
		return err
	}
	for i, item := range paths {
		if item.Path != selected {
			continue
		}
		choice := "All pages"
		if item.Publish == config.PublicationExplicit {
			choice = "Explicit pages only"
		}
		if err := survey.AskOne(&survey.Select{Message: "Publish:", Options: []string{"All pages", "Explicit pages only"}, Default: choice}, &choice, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
			return err
		}
		if choice == "All pages" {
			paths[i].Publish = config.PublicationAll
		} else {
			paths[i].Publish = config.PublicationExplicit
		}
		return nil
	}
	return fmt.Errorf("selected path is unavailable")
}

func removePublicationPaths(paths []config.PublicationPath) ([]config.PublicationPath, error) {
	options := make([]string, len(paths))
	for i, item := range paths {
		options[i] = item.Path
	}
	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{Message: "Remove publication paths:", Options: options}, &selected, survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)); err != nil {
		return nil, err
	}
	removed := make(map[string]bool, len(selected))
	for _, value := range selected {
		removed[value] = true
	}
	result := make([]config.PublicationPath, 0, len(paths))
	for _, item := range paths {
		if !removed[item.Path] {
			result = append(result, item)
		}
	}
	return result, nil
}

func integrationLabel(name integration.Integration) string {
	if name == integration.StandardSite {
		return "Standard.site"
	}
	return "Bluesky"
}

func init() { rootCmd.AddCommand(configureCmd) }
