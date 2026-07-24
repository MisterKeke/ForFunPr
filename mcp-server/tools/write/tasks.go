package writetools

import (
	"context"
	"fmt"

	"currency-wails/mcp-server/schemas"
	"currency-wails/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTasks adds create, overwrite, toggle, and delete task tools.
func RegisterTasks(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "create_task",
		Title:       "Create task",
		Description: "Create a new local task. Repeated calls create additional tasks.",
		InputSchema: schemas.CreateTaskInputSchema,
		Annotations: tools.WriteAnnotations(false, false, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.CreateTaskInput,
	) (*mcp.CallToolResult, schemas.TasksOutput, error) {
		title, description, priority, dueDate, err := normalizeTaskWrite(
			input.Title,
			input.Description,
			input.Priority,
			input.DueDate,
		)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}

		args := []string{"tasks", "create", "--title", title}
		args = appendTaskOptionalFlags(
			args,
			description,
			priority,
			dueDate,
		)
		return runTaskMutation(
			ctx,
			runner,
			args,
			"Created a local task.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "update_task",
		Title:       "Update task",
		Description: "Overwrite a local task's title, description, priority, and due date.",
		InputSchema: schemas.UpdateTaskInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.UpdateTaskInput,
	) (*mcp.CallToolResult, schemas.TasksOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		title, description, priority, dueDate, err := normalizeTaskWrite(
			input.Title,
			input.Description,
			input.Priority,
			input.DueDate,
		)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}

		args := []string{
			"tasks", "update",
			"--id", positiveInteger(id),
			"--title", title,
		}
		args = appendTaskOptionalFlags(
			args,
			description,
			priority,
			dueDate,
		)
		return runTaskMutation(
			ctx,
			runner,
			args,
			"Updated the requested local task.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "toggle_task",
		Title:       "Toggle task completion",
		Description: "Toggle a local task between complete and incomplete. This operation is non-idempotent.",
		InputSchema: schemas.TaskIDInputSchema,
		Annotations: tools.WriteAnnotations(true, false, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.TaskIDInput,
	) (*mcp.CallToolResult, schemas.TasksOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		return runTaskMutation(
			ctx,
			runner,
			[]string{
				"tasks", "toggle",
				"--id", positiveInteger(id),
			},
			"Toggled completion for the requested local task.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "delete_task",
		Title:       "Delete task",
		Description: "Permanently delete one local task.",
		InputSchema: schemas.TaskIDInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.TaskIDInput,
	) (*mcp.CallToolResult, schemas.TasksOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		return runTaskMutation(
			ctx,
			runner,
			[]string{
				"tasks", "delete",
				"--id", positiveInteger(id),
			},
			"Deleted the requested local task.",
		)
	})
}

func normalizeTaskWrite(
	titleValue string,
	descriptionValue string,
	priorityValue string,
	dueDateValue string,
) (string, string, string, string, error) {
	title, err := schemas.RequiredString("title", titleValue)
	if err != nil {
		return "", "", "", "", err
	}
	priority, err := schemas.OptionalPriority(priorityValue)
	if err != nil {
		return "", "", "", "", err
	}
	dueDate, err := schemas.OptionalDate("due_date", dueDateValue)
	if err != nil {
		return "", "", "", "", err
	}
	return title,
		schemas.OptionalString(descriptionValue),
		priority,
		dueDate,
		nil
}

func appendTaskOptionalFlags(
	args []string,
	description string,
	priority string,
	dueDate string,
) []string {
	args = tools.OptionalStringFlag(args, "--description", description)
	args = tools.OptionalStringFlag(args, "--priority", priority)
	return tools.OptionalStringFlag(args, "--due-date", dueDate)
}

func runTaskMutation(
	ctx context.Context,
	runner *tools.Runner,
	args []string,
	summary string,
) (*mcp.CallToolResult, schemas.TasksOutput, error) {
	items, err := tools.Run[[]schemas.Task](ctx, runner, args)
	if err == nil {
		summary = fmt.Sprintf("%s %d tasks now exist.", summary, len(items))
	}
	return tools.Response(
		summary,
		schemas.TasksOutput{Tasks: items},
		err,
	)
}
