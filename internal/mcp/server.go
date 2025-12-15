// ABOUTME: MCP server setup and initialization.
// ABOUTME: Wires together tools, resources, and Pushover client.
package mcp

import (
	"context"
	"fmt"

	"github.com/harper/push/internal/config"
	"github.com/harper/push/internal/db"
	"github.com/harper/push/internal/pushover"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP runtime and Push integrations.
type Server struct {
	mcp     *mcp.Server
	cfg     *config.Config
	cfgPath string
	store   *db.Store
	dbPath  string
}

// NewServer sets up the MCP server with all tools and resources.
func NewServer(cfg *config.Config, cfgPath string, store *db.Store, dbPath string) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if store == nil {
		return nil, fmt.Errorf("database store is required")
	}

	impl := &mcp.Implementation{Name: "push", Version: "1.0.0"}
	srv := mcp.NewServer(impl, nil)

	server := &Server{
		mcp:     srv,
		cfg:     cfg,
		cfgPath: cfgPath,
		store:   store,
		dbPath:  dbPath,
	}

	server.registerTools()
	server.registerResources()

	return server, nil
}

// Serve starts the MCP server over stdio.
func (s *Server) Serve(ctx context.Context) error {
	transport := &mcp.StdioTransport{}
	return s.mcp.Run(ctx, transport)
}

func (s *Server) newClient() *pushover.Client {
	cfg := s.cfg
	if cfg == nil {
		return pushover.NewClient("", "", "", "")
	}
	return pushover.NewClient(cfg.AppToken, cfg.UserKey, cfg.DeviceID, cfg.DeviceSecret)
}
