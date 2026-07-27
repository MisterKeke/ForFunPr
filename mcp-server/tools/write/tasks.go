package writetools

import (
	"context"
	"encoding/json"
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
		difficulty, tags, subtasks, err := normalizeTaskMetadata(
			input.Difficulty, input.Tags, input.Subtasks,
		)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		for _, subtask := range subtasks {
			if subtask.ID != 0 {
				return nil, schemas.TasksOutput{}, fmt.Errorf("new task subtasks cannot include IDs")
			}
		}

		args := []string{"tasks", "create", "--title", title}
		args = appendTaskOptionalFlags(
			args,
			description,
			priority,
			dueDate,
		)
		args, err = appendTaskMetadataFlags(args, difficulty, tags, subtasks)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
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
		Description: "Overwrite a local task's core fields and optionally replace its difficulty, tags, and subtasks.",
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
		if input.ClearDifficulty && input.Difficulty != "" {
			return nil, schemas.TasksOutput{}, fmt.Errorf("difficulty and clear_difficulty cannot both be set")
		}
		if input.ClearTags && input.Tags != nil {
			return nil, schemas.TasksOutput{}, fmt.Errorf("tags and clear_tags cannot both be set")
		}
		if input.ClearSubtasks && input.Subtasks != nil {
			return nil, schemas.TasksOutput{}, fmt.Errorf("subtasks and clear_subtasks cannot both be set")
		}
		difficulty, tags, subtasks, err := normalizeTaskMetadata(
			input.Difficulty, input.Tags, input.Subtasks,
		)
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
		args, err = appendTaskMetadataFlags(args, difficulty, tags, subtasks)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		if input.ClearDifficulty {
			args = append(args, "--clear-difficulty")
		}
		if input.ClearTags {
			args = append(args, "--clear-tags")
		}
		if input.ClearSubtasks {
			args = append(args, "--clear-subtasks")
		}
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

	tools.AddTool(server, &mcp.Tool{
		Name:        "toggle_task_subtask",
		Title:       "Toggle task subtask completion",
		Description: "Toggle one subtask belonging to a hard local task.",
		InputSchema: schemas.TaskSubtaskIDInputSchema,
		Annotations: tools.WriteAnnotations(true, false, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.TaskSubtaskIDInput,
	) (*mcp.CallToolResult, schemas.TasksOutput, error) {
		taskID, err := schemas.PositiveID("task_id", input.TaskID)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		subtaskID, err := schemas.PositiveID("subtask_id", input.SubtaskID)
		if err != nil {
			return nil, schemas.TasksOutput{}, err
		}
		return runTaskMutation(
			ctx,
			runner,
			[]string{
				"tasks", "toggle-subtask",
				"--task-id", positiveInteger(taskID),
				"--subtask-id", positiveInteger(subtaskID),
			},
			"Toggled completion for the requested local subtask.",
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

func normalizeTaskMetadata(
	difficultyValue string,
	tagValues []string,
	subtaskValues []schemas.TaskSubtaskInput,
) (string, []string, []schemas.TaskSubtaskInput, error) {
	difficulty, err := schemas.OptionalDifficulty(difficultyValue)
	if err != nil {
		return "", nil, nil, err
	}
	var tags []string
	if tagValues != nil {
		tags, err = schemas.OptionalTags(tagValues)
		if err != nil {
			return "", nil, nil, err
		}
	}
	var subtasks []schemas.TaskSubtaskInput
	if subtaskValues != nil {
		subtasks = make([]schemas.TaskSubtaskInput, 0, len(subtaskValues))
	}
	seenIDs := make(map[int]struct{}, len(subtaskValues))
	for index, value := range subtaskValues {
		title, err := schemas.RequiredString("subtask.title", value.Title)
		if err != nil {
			return "", nil, nil, err
		}
		if value.ID < 0 {
			return "", nil, nil, fmt.Errorf("subtask IDs cannot be negative")
		}
		if value.ID > 0 {
			if _, exists := seenIDs[value.ID]; exists {
				return "", nil, nil, fmt.Errorf("subtask IDs cannot be repeated")
			}
			seenIDs[value.ID] = struct{}{}
		}
		value.Title = title
		value.Position = index
		subtasks = append(subtasks, value)
	}
	if len(subtasks) > 0 && difficulty != "" && difficulty != "hard" {
		return "", nil, nil, fmt.Errorf("subtasks are only allowed when difficulty is hard")
	}
	return difficulty, tags, subtasks, nil
}

func appendTaskMetadataFlags(
	args []string,
	difficulty string,
	tags []string,
	subtasks []schemas.TaskSubtaskInput,
) ([]string, error) {
	args = tools.OptionalStringFlag(args, "--difficulty", difficulty)
	if tags != nil {
		if len(tags) == 0 {
			args = append(args, "--clear-tags")
		} else {
			for _, tag := range tags {
				args = append(args, "--tag", tag)
			}
		}
	}
	if subtasks != nil {
		encoded, err := json.Marshal(subtasks)
		if err != nil {
			return nil, fmt.Errorf("encode subtasks: %w", err)
		}
		args = append(args, "--subtasks-json", string(encoded))
	}
	return args, nil
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
