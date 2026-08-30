package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandNamespaces(t *testing.T) {
	rootNames := commandNames(rootCmd.Commands())
	for _, name := range []string{"init", "scan", "atproto"} {
		if !rootNames[name] {
			t.Errorf("root command %q is missing", name)
		}
	}
	for _, name := range []string{"login", "logout", "auth", "identity"} {
		if rootNames[name] {
			t.Errorf("ATProto command %q is still registered at root", name)
		}
	}

	atprotoNames := commandNames(atprotoCmd.Commands())
	for _, name := range []string{"login", "logout", "auth", "identity"} {
		if !atprotoNames[name] {
			t.Errorf("ATProto command %q is missing", name)
		}
	}

	authNames := commandNames(authCmd.Commands())
	for _, name := range []string{"status", "list", "switch"} {
		if !authNames[name] {
			t.Errorf("ATProto auth command %q is missing", name)
		}
	}
}

func TestRootHelpDescribesProtocolNamespace(t *testing.T) {
	var output bytes.Buffer
	rootCmd.SetOut(&output)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	if err := rootCmd.Help(); err != nil {
		t.Fatalf("rootCmd.Help() unexpected error = %v", err)
	}
	help := output.String()
	if !strings.Contains(help, "atproto") {
		t.Errorf("root help does not contain atproto: %q", help)
	}
	for _, name := range []string{"login", "logout", "auth"} {
		if strings.Contains(help, "\n  "+name+" ") {
			t.Errorf("root help advertises ATProto command %q: %q", name, help)
		}
	}
}

func commandNames(commands []*cobra.Command) map[string]bool {
	result := make(map[string]bool, len(commands))
	for _, command := range commands {
		result[command.Name()] = true
	}
	return result
}
