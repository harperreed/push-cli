// ABOUTME: MCP resource definitions and providers.
// ABOUTME: Exposes unread messages, history, and status as resources.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ResourcePayload struct {
	Metadata ResourceMetadata  `json:"metadata"`
	Data     interface{}       `json:"data"`
	Links    map[string]string `json:"links,omitempty"`
}

type ResourceMetadata struct {
	Timestamp   time.Time `json:"timestamp"`
	ResourceURI string    `json:"resource_uri"`
	Count       int       `json:"count"`
}

func (s *Server) registerResources() {
	s.registerUnreadResource()
	s.registerHistoryResource()
	s.registerStatusResource()
}

func (s *Server) registerUnreadResource() {
	res := &mcp.Resource{
		URI:         "push://unread",
		Name:        "Unread Messages",
		Description: "Current unread messages fetched directly from Pushover (no persistence or acknowledgement).",
		MIMEType:    "application/json",
	}

	s.mcp.AddResource(res, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if err := s.cfg.ValidateReceive(); err != nil {
			return nil, err
		}
		client := s.newClient()
		result, err := client.FetchMessages(ctx)
		if err != nil {
			return nil, err
		}
		payload := ResourcePayload{
			Metadata: ResourceMetadata{
				Timestamp:   time.Now(),
				ResourceURI: res.URI,
				Count:       len(result.Messages),
			},
			Data: result.Messages,
		}
		return buildResourceResult(req.Params.URI, payload)
	})
}

func (s *Server) registerHistoryResource() {
	res := &mcp.Resource{
		URI:         "push://history",
		Name:        "Recent History",
		Description: "Last 20 persisted messages from the local SQLite database.",
		MIMEType:    "application/json",
	}

	s.mcp.AddResource(res, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		records, err := s.store.QueryMessages(ctx, 20, nil, "")
		if err != nil {
			return nil, err
		}
		payload := ResourcePayload{
			Metadata: ResourceMetadata{
				Timestamp:   time.Now(),
				ResourceURI: res.URI,
				Count:       len(records),
			},
			Data: records,
		}
		return buildResourceResult(req.Params.URI, payload)
	})
}

func (s *Server) registerStatusResource() {
	res := &mcp.Resource{
		URI:         "push://status",
		Name:        "Push Status",
		Description: "Credential and database health summary for the Push CLI.",
		MIMEType:    "application/json",
	}

	s.mcp.AddResource(res, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		cfg := s.cfg
		status := map[string]interface{}{
			"config": map[string]interface{}{
				"path":              s.cfgPath,
				"has_app_token":     cfg.AppToken != "",
				"has_user_key":      cfg.UserKey != "",
				"device_configured": cfg.DeviceConfigured(),
				"default_device":    cfg.DefaultDevice,
				"default_priority":  cfg.DefaultPriority,
			},
			"database": map[string]interface{}{
				"path": s.dbPath,
			},
			"timestamp": time.Now(),
		}

		payload := ResourcePayload{
			Metadata: ResourceMetadata{
				Timestamp:   time.Now(),
				ResourceURI: res.URI,
				Count:       1,
			},
			Data: status,
			Links: map[string]string{
				"history": "push://history",
				"unread":  "push://unread",
			},
		}
		return buildResourceResult(req.Params.URI, payload)
	})
}

func buildResourceResult(uri string, payload ResourcePayload) (*mcp.ReadResourceResult, error) {
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode resource: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(bytes),
			},
		},
	}, nil
}
