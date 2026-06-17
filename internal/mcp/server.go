package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "silo"
	serverVersion = "0.2.0"
)

// NewServer creates an MCP server with all tools registered.
// Dependencies must be set via SetDeps before the server starts.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(false),
	)

	// Profile tools.
	s.AddTool(getProfileContextTool(), handleGetProfileContext)
	s.AddTool(initProfileTool(), handleInitProfile)

	// Recommend tool.
	s.AddTool(siloRecommendTool(), handleSiloRecommend)

	return s
}

// ServeStdio starts the MCP server over stdio transport.
// This is a convenience wrapper around server.ServeStdio.
func ServeStdio(s *server.MCPServer) error {
	return server.ServeStdio(s)
}
