// ABOUTME: MCP command for starting the Model Context Protocol server.
// ABOUTME: Exposes push capabilities as MCP tools over stdio.
package cli

import (
	"fmt"

	pushmcp "github.com/harper/push/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server",
		RunE:  runMCP,
	}
	return cmd
}

func runMCP(cmd *cobra.Command, args []string) error {
	cfg, cfgPath, err := loadConfig()
	if err != nil {
		return err
	}

	if err := cfg.ValidateSend(); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
	}
	if !cfg.DeviceConfigured() {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: device not configured, check_messages and mark_read will fail until you run 'push login'\n")
	}

	store, dbPath, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	server, err := pushmcp.NewServer(cfg, cfgPath, store, dbPath)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Starting MCP server (stdio)...")
	return server.Serve(cmd.Context())
}
