package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddTool registers a typed MCP tool with an explicit input schema and an
// inferred typed output schema.
func AddTool[In, Out any](
	server *mcp.Server,
	tool *mcp.Tool,
	handler mcp.ToolHandlerFor[In, Out],
) {
	if tool.InputSchema == nil {
		panic(fmt.Sprintf("tool %q has no explicit input schema", tool.Name))
	}
	mcp.AddTool(server, tool, handler)
}

// Execute runs a CLI command and returns both a concise text block and typed
// structured content.
func Execute[Out any](
	ctx context.Context,
	runner *Runner,
	args []string,
	summary string,
) (*mcp.CallToolResult, Out, error) {
	output, err := Run[Out](ctx, runner, args)
	return Response(summary, output, err)
}

// Response converts a typed CLI result into an MCP tool response.
func Response[Out any](
	summary string,
	output Out,
	err error,
) (*mcp.CallToolResult, Out, error) {
	if err != nil {
		var zero Out
		return nil, zero, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: summary},
		},
	}, output, nil
}

// OptionalStringFlag appends a non-empty optional CLI string flag.
func OptionalStringFlag(args []string, name string, value string) []string {
	if value == "" {
		return args
	}
	return append(args, name, value)
}

// OptionalIntFlag appends an optional integer CLI flag, preserving zero.
func OptionalIntFlag(args []string, name string, value *int) []string {
	if value == nil {
		return args
	}
	return append(args, name, strconv.Itoa(*value))
}

// ReadAnnotations returns annotations for a GET-backed tool.
func ReadAnnotations(openWorld bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: boolPointer(false),
		OpenWorldHint:   boolPointer(openWorld),
	}
}

// WriteAnnotations returns annotations for a mutation-backed tool.
func WriteAnnotations(
	destructive bool,
	idempotent bool,
	openWorld bool,
) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: boolPointer(destructive),
		IdempotentHint:  idempotent,
		OpenWorldHint:   boolPointer(openWorld),
	}
}

func boolPointer(value bool) *bool {
	return &value
}
