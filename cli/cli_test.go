package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVersionCommand(t *testing.T) {
	var stdout bytes.Buffer
	err := Execute(context.Background(), Options{
		Name:        "sample",
		Args:        []string{"version"},
		Stdout:      &stdout,
		VersionText: func() string { return "sample dev\n" },
	})
	if err != nil {
		t.Fatalf("execute version command: %v", err)
	}
	if got := stdout.String(); got != "sample dev\n" {
		t.Fatalf("unexpected version output: %q", got)
	}
}

func TestCustomCommandReceivesCLIContext(t *testing.T) {
	ctx := context.Background()
	var stdout bytes.Buffer
	err := Execute(ctx, Options{
		Name:   "sample",
		Args:   []string{"--config", "custom.yaml", "debug", "status"},
		Stdout: &stdout,
		Configure: func(root *cobra.Command, c *Context) {
			root.AddCommand(Command(ctx, c, "debug <name>", "debug command", func(ctx context.Context, c *Context, cmd *cobra.Command, args []string) error {
				if len(args) != 1 {
					return fmt.Errorf("expected one arg")
				}
				_, _ = fmt.Fprintf(c.Stdout, "%s:%s", c.ConfigPath, args[0])
				return nil
			}))
		},
	})
	if err != nil {
		t.Fatalf("execute custom command: %v", err)
	}
	if got := stdout.String(); got != "custom.yaml:status" {
		t.Fatalf("unexpected custom command output: %q", got)
	}
}

func TestServerCommandRequiresServer(t *testing.T) {
	err := Execute(context.Background(), Options{
		Name: "sample",
		Args: []string{"server"},
	})
	if err == nil || !strings.Contains(err.Error(), "server command is not configured") {
		t.Fatalf("expected unconfigured server error, got %v", err)
	}
}
