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
	) (*mcp.CallToolResult, schemas.TaskListOutput, error) {
		query := schemas.OptionalString(input.Query)
		date, err := schemas.OptionalDate("date", input.Date)
		if err != nil {
			return nil, schemas.TaskListOutput{}, err
		}
		priority, err := schemas.OptionalPriority(input.Priority)
		if err != nil {
			return nil, schemas.TaskListOutput{}, err
		}
		difficulty, err := schemas.OptionalDifficultyFilter(input.Difficulty)
		if err != nil {
			return nil, schemas.TaskListOutput{}, err
		}
		var tags []string
		if input.Tags != nil {
			tags, err = schemas.OptionalTags(input.Tags)
			if err != nil {
				return nil, schemas.TaskListOutput{}, err
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
		args = tools.OptionalStringFlag(args, "--due-from", schemas.OptionalString(input.DueFrom))
		args = tools.OptionalStringFlag(args, "--due-to", schemas.OptionalString(input.DueTo))
		args = tools.OptionalStringFlag(args, "--completion", schemas.OptionalString(input.Completion))
		args = tools.OptionalStringFlag(args, "--sort", schemas.OptionalString(input.Sort))
		args = tools.OptionalStringFlag(args, "--direction", schemas.OptionalString(input.Direction))
		args = tools.OptionalIntFlag(args, "--limit", input.Limit)
		args = tools.OptionalIntFlag(args, "--offset", input.Offset)
		if input.Overdue {
			args = append(args, "--overdue")
		}
		if input.Undated {
			args = append(args, "--undated")
		}
		output, err := tools.Run[schemas.TaskListOutput](ctx, runner, args)
		return tools.Response(
			fmt.Sprintf("Listed %d of %d tasks.", len(output.Items), output.Total),
			output,
			err,
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "list_today_tasks",
		Title:       "List today's tasks",
		Description: "List today's incomplete local tasks.",
		InputSchema: schemas.TaskDateInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.TaskDateInput,
	) (*mcp.CallToolResult, schemas.TodayTasksOutput, error) {
		args := []string{"tasks", "today"}
		if input.IncludeOverdue {
			args = append(args, "--include-overdue")
		}
		if input.IncludeUndated {
			args = append(args, "--include-undated")
		}
		output, err := tools.Run[schemas.TodayTasksOutput](
			ctx,
			runner,
			args,
		)
		return tools.Response(
			fmt.Sprintf("Listed %d incomplete tasks due today.", len(output.DueToday)),
			output,
			err,
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "list_this_week_tasks",
		Title:       "List this week's remaining tasks",
		Description: "List incomplete local tasks due after today through the end of the current local week.",
		InputSchema: schemas.TaskDateInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.TaskDateInput,
	) (*mcp.CallToolResult, schemas.WeekTasksOutput, error) {
		args := []string{"tasks", "week"}
		if input.IncludeOverdue {
			args = append(args, "--include-overdue")
		}
		if input.IncludeUndated {
			args = append(args, "--include-undated")
		}
		args = tools.OptionalIntFlag(args, "--week-start", input.WeekStart)
		output, err := tools.Run[schemas.WeekTasksOutput](ctx, runner, args)
		return tools.Response(
			fmt.Sprintf("Listed %d incomplete tasks due later this week.", len(output.Items)),
			output,
			err,
		)
	})
}
