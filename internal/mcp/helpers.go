package mcp

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/engram"
)

// Deps holds the dependencies injected into MCP handlers.
type Deps struct {
	Config *config.Config
	Engram engram.Client
}

// SetDeps stores the package-level dependencies for handler use.
// Must be called before the server starts handling requests.
func SetDeps(d Deps) { deps = d }

var deps Deps

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func stringSliceArg(req mcp.CallToolRequest, name string) []string {
	args, _ := req.Params.Arguments.(map[string]any)
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func numberArg(req mcp.CallToolRequest, name string) (float64, error) {
	args, _ := req.Params.Arguments.(map[string]any)
	raw, ok := args[name]
	if !ok || raw == nil {
		return 0, errMissing(name)
	}
	switch v := raw.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, errNotNumber(name)
	}
}

func errMissing(name string) error {
	return &argError{msg: "missing required argument: " + name}
}

func errNotNumber(name string) error {
	return &argError{msg: "argument " + name + " must be a number"}
}

type argError struct{ msg string }

func (e *argError) Error() string { return e.msg }
