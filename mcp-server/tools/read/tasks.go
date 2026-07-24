package readtools

import (
	"context"
	"fmt"

	"currency-wails/mcp-server/schemas"
	"currency-wails/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTasks adds local task query tools.
func RegisterTasks(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "list_tasks",
		Title:       "List tasks",
		Description: "List local tasks, optionally filtering by a YYYY-MM-DD due date.",
		InputSchema: schemas.TaskListInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.TaskListInput,
	) (*mcp.CallToolResult, schemas.TasksOutput, error) {
		date, err := schemas.OptionalDate("date", input.Date)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}

		args := []string{"tasks", "list"}
		args = tools.OptionalStringFlag(args, "--date", date)
		items, err := tools.Run[[]schemas.Task](ctx, runner, args)
		return tools.Response(
			fmt.Sprintf("Listed %d tasks.", len(items)),
			schemas.TasksOutput{Tasks: items},
			err,
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "list_today_tasks",
		Title:       "List today's tasks",
		Description: "List today's incomplete local tasks.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.TasksOutput, error) {
		items, err := tools.Run[[]schemas.Task](
			ctx,
			runner,
			[]string{"tasks", "today"},
		)
		return tools.Response(
			fmt.Sprintf("Listed %d incomplete tasks due today.", len(items)),
			schemas.TasksOutput{Tasks: items},
			err,
		)
	})
}
