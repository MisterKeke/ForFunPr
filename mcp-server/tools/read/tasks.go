package readtools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTasks adds local task query tools.
func RegisterTasks(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "list_tasks",
		Title:       "List tasks",
		Description: "Search local tasks and filter them by due date, priority, difficulty, and exact tags.",
		InputSchema: schemas.TaskListInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.TaskListInput,
	) (*mcp.CallToolResult, schemas.TasksOutput, error) {
		query := schemas.OptionalString(input.Query)
		date, err := schemas.OptionalDate("date", input.Date)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		priority, err := schemas.OptionalPriority(input.Priority)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		difficulty, err := schemas.OptionalDifficultyFilter(input.Difficulty)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		var tags []string
		if input.Tags != nil {
			tags, err = schemas.OptionalTags(input.Tags)
			if err != nil {
				return nil, schemas.TasksOutput{}, err
			}
		}

		args := []string{"tasks", "list"}
		args = tools.OptionalStringFlag(args, "--search", query)
		args = tools.OptionalStringFlag(args, "--date", date)
		args = tools.OptionalStringFlag(args, "--priority", priority)
		args = tools.OptionalStringFlag(args, "--difficulty", difficulty)
		for _, tag := range tags {
			args = append(args, "--tag", tag)
		}
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
