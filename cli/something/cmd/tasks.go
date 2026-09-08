package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newTasksCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "tasks",
		Short: "Read and modify tasks",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newTaskListCommand(dependencies),
		newTaskTodayCommand(dependencies),
		newTaskWeekCommand(dependencies),
		newTaskCreateCommand(dependencies),
		newTaskUpdateCommand(dependencies),
		newTaskToggleCommand(dependencies),
		newTaskSubtaskToggleCommand(dependencies),
		newTaskDeleteCommand(dependencies),
	)
	return command
}

func newTaskWeekCommand(dependencies commandDependencies) *cobra.Command {
	var options apiclient.TaskDateQuery
	var weekStart int
	command := &cobra.Command{
		Use:   "week",
		Short: "List incomplete tasks due later this week",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			if command.Flags().Changed("week-start") {
				options.WeekStart = &weekStart
			}
			result, err := client.WeekTasks(command.Context(), options)
			if err != nil {
				return fmt.Errorf("list this week's tasks: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().BoolVar(&options.IncludeOverdue, "include-overdue", false, "include overdue tasks in a separate group")
	command.Flags().BoolVar(&options.IncludeUndated, "include-undated", false, "include tasks without due dates in a separate group")
	command.Flags().IntVar(&weekStart, "week-start", 1, "week start day: 0 Sunday through 6 Saturday")
	return command
}

func newTaskListCommand(dependencies commandDependencies) *cobra.Command {
	var filter apiclient.TaskListFilter

	command := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.ListTasks(command.Context(), filter)
			if err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}

			if dependencies.outputFormat() == "json" {
				return dependencies.writeValue(command, result)
			}
			return dependencies.writeValue(command, result.Items)
		},
	}
	command.Flags().StringVar(
		&filter.Query,
		"search",
		"",
		"search task titles, descriptions, tags, and subtasks",
	)
	command.Flags().StringVar(
		&filter.Date,
		"date",
		"",
		"only list tasks due on YYYY-MM-DD",
	)
	command.Flags().StringVar(
		&filter.Priority,
		"priority",
		"",
		"only list tasks with priority low, medium, or high",
	)
	command.Flags().StringVar(
		&filter.Difficulty,
		"difficulty",
		"",
		"only list tasks with difficulty unset, easy, medium, or hard",
	)
	command.Flags().StringSliceVar(
		&filter.Tags,
		"tag",
		nil,
		"required task tag; may be repeated and all tags must match",
	)
	command.Flags().StringVar(&filter.DueFrom, "due-from", "", "only tasks due on or after YYYY-MM-DD")
	command.Flags().StringVar(&filter.DueTo, "due-to", "", "only tasks due on or before YYYY-MM-DD")
	command.Flags().StringVar(&filter.Completion, "completion", "all", "all, complete, or incomplete")
	command.Flags().BoolVar(&filter.Overdue, "overdue", false, "only incomplete overdue tasks")
	command.Flags().BoolVar(&filter.Undated, "undated", false, "only tasks without a due date")
	command.Flags().StringVar(&filter.Sort, "sort", "created_at", "created_at, updated_at, due_date, priority, title, or id")
	command.Flags().StringVar(&filter.Direction, "direction", "desc", "asc or desc")
	command.Flags().IntVar(&filter.Limit, "limit", 100, "maximum tasks to return (1-200)")
	command.Flags().IntVar(&filter.Offset, "offset", 0, "tasks to skip")
	return command
}

func newTaskTodayCommand(dependencies commandDependencies) *cobra.Command {
	var options apiclient.TaskDateQuery
	command := &cobra.Command{
		Use:   "today",
		Short: "List today's incomplete tasks",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.TodayTasks(command.Context(), options)
			if err != nil {
				return fmt.Errorf("list today's tasks: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().BoolVar(&options.IncludeOverdue, "include-overdue", false, "include overdue tasks in a separate group")
	command.Flags().BoolVar(&options.IncludeUndated, "include-undated", false, "include tasks without due dates in a separate group")
	return command
}

func newTaskCreateCommand(dependencies commandDependencies) *cobra.Command {
	var request apiclient.TaskWriteRequest
	var metadata taskMetadataFlags

	command := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Title", &request.Title); err != nil {
				return err
			}

			if err := applyTaskMetadataFlags(command, &request, &metadata); err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.CreateTask(command.Context(), request)
			if err != nil {
				return fmt.Errorf("create task: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	addTaskWriteFlags(command, &request, &metadata)
	return command
}

func newTaskUpdateCommand(dependencies commandDependencies) *cobra.Command {
	var idText string
	var expectedRevision int
	var request apiclient.TaskWriteRequest
	var metadata taskMetadataFlags

	command := &cobra.Command{
		Use:   "update",
		Short: "Update a task",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Task ID", &idText); err != nil {
				return err
			}
			if err := prompt.required("Title", &request.Title); err != nil {
				return err
			}

			id, err := parseInteger("Task ID", idText)
			if err != nil {
				return err
			}

			if err := applyTaskMetadataFlags(command, &request, &metadata); err != nil {
				return err
			}
			if command.Flags().Changed("expected-revision") {
				if expectedRevision <= 0 {
					return fmt.Errorf("expected revision must be positive")
				}
				request.ExpectedRevision = &expectedRevision
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.UpdateTask(
				command.Context(),
				id,
				request,
			)
			if err != nil {
				return fmt.Errorf("update task: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "task ID")
	command.Flags().IntVar(&expectedRevision, "expected-revision", 0, "reject the update if the task revision changed")
	addTaskWriteFlags(command, &request, &metadata)
	return command
}

func newTaskToggleCommand(dependencies commandDependencies) *cobra.Command {
	return newTaskIDCommand(
		dependencies,
		"toggle",
		"Toggle whether a task is complete",
		func(
			client *apiclient.Client,
			ctx context.Context,
			id int,
			expectedRevision *int,
		) (any, error) {
			return client.ToggleTask(ctx, id, expectedRevision)
		},
	)
}

func newTaskSubtaskToggleCommand(dependencies commandDependencies) *cobra.Command {
	var taskIDText string
	var subtaskIDText string
	var expectedRevision int
	command := &cobra.Command{
		Use:   "toggle-subtask",
		Short: "Toggle whether a hard-task subtask is complete",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Task ID", &taskIDText); err != nil {
				return err
			}
			if err := prompt.required("Subtask ID", &subtaskIDText); err != nil {
				return err
			}
			taskID, err := parseInteger("Task ID", taskIDText)
			if err != nil {
				return err
			}
			subtaskID, err := parseInteger("Subtask ID", subtaskIDText)
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			var expected *int
			if command.Flags().Changed("expected-revision") {
				if expectedRevision <= 0 {
					return fmt.Errorf("expected revision must be positive")
				}
				expected = &expectedRevision
			}
			result, err := client.ToggleTaskSubtask(command.Context(), taskID, subtaskID, expected)
			if err != nil {
				return fmt.Errorf("toggle task subtask: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&taskIDText, "task-id", "", "task ID")
	command.Flags().StringVar(&subtaskIDText, "subtask-id", "", "subtask ID")
	command.Flags().IntVar(&expectedRevision, "expected-revision", 0, "reject the toggle if the task revision changed")
	return command
}

func newTaskDeleteCommand(dependencies commandDependencies) *cobra.Command {
	return newTaskIDCommand(
		dependencies,
		"delete",
		"Delete a task",
		func(
			client *apiclient.Client,
			ctx context.Context,
			id int,
			expectedRevision *int,
		) (any, error) {
			return client.DeleteTask(ctx, id, expectedRevision)
		},
	)
}

func newTaskIDCommand(
	dependencies commandDependencies,
	use string,
	short string,
	call func(
		*apiclient.Client,
		context.Context,
		int,
		*int,
	) (any, error),
) *cobra.Command {
	var idText string
	var expectedRevision int

	command := &cobra.Command{
		Use:   use + " [TASK_ID]",
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			if idText == "" && len(arguments) == 1 {
				idText = arguments[0]
			}

			prompt := newPrompter(command)
			if err := prompt.required("Task ID", &idText); err != nil {
				return err
			}

			id, err := parseInteger("Task ID", idText)
			if err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			var expected *int
			if command.Flags().Changed("expected-revision") {
				if expectedRevision <= 0 {
					return fmt.Errorf("expected revision must be positive")
				}
				expected = &expectedRevision
			}
			result, err := call(client, command.Context(), id, expected)
			if err != nil {
				return fmt.Errorf("%s task: %w", use, err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "task ID")
	command.Flags().IntVar(&expectedRevision, "expected-revision", 0, "reject the mutation if the task revision changed")
	return command
}

func addTaskWriteFlags(
	command *cobra.Command,
	request *apiclient.TaskWriteRequest,
	metadata *taskMetadataFlags,
) {
	command.Flags().StringVar(&request.Title, "title", "", "task title")
	command.Flags().StringVar(
		&request.Description,
		"description",
		"",
		"task description",
	)
	command.Flags().StringVar(
		&request.Priority,
		"priority",
		"",
		"task priority: low, medium, or high",
	)
	command.Flags().StringVar(
		&request.DueDate,
		"due-date",
		"",
		"task due date in YYYY-MM-DD format",
	)
	command.Flags().StringVar(
		&metadata.Difficulty,
		"difficulty",
		"",
		"task difficulty: easy, medium, or hard",
	)
	command.Flags().StringSliceVar(
		&metadata.Tags,
		"tag",
		nil,
		"task tag; may be repeated",
	)
	command.Flags().StringArrayVar(
		&metadata.Subtasks,
		"subtask",
		nil,
		"replacement hard-task subtask title; may be repeated",
	)
	command.Flags().StringVar(
		&metadata.SubtasksJSON,
		"subtasks-json",
		"",
		"replacement subtask JSON array preserving IDs and completion state",
	)
	command.Flags().BoolVar(&metadata.ClearDifficulty, "clear-difficulty", false, "clear task difficulty")
	command.Flags().BoolVar(&metadata.ClearTags, "clear-tags", false, "remove every task tag")
	command.Flags().BoolVar(&metadata.ClearSubtasks, "clear-subtasks", false, "remove every task subtask")
}

type taskMetadataFlags struct {
	Difficulty      string
	Tags            []string
	Subtasks        []string
	SubtasksJSON    string
	ClearDifficulty bool
	ClearTags       bool
	ClearSubtasks   bool
}

func applyTaskMetadataFlags(
	command *cobra.Command,
	request *apiclient.TaskWriteRequest,
	metadata *taskMetadataFlags,
) error {
	if metadata.ClearDifficulty && command.Flags().Changed("difficulty") {
		return fmt.Errorf("--difficulty and --clear-difficulty cannot be used together")
	}
	if metadata.ClearTags && command.Flags().Changed("tag") {
		return fmt.Errorf("--tag and --clear-tags cannot be used together")
	}
	if metadata.ClearSubtasks && command.Flags().Changed("subtask") {
		return fmt.Errorf("--subtask and --clear-subtasks cannot be used together")
	}
	if command.Flags().Changed("subtask") && command.Flags().Changed("subtasks-json") {
		return fmt.Errorf("--subtask and --subtasks-json cannot be used together")
	}
	if metadata.ClearSubtasks && command.Flags().Changed("subtasks-json") {
		return fmt.Errorf("--subtasks-json and --clear-subtasks cannot be used together")
	}
	if metadata.ClearDifficulty || command.Flags().Changed("difficulty") {
		value := metadata.Difficulty
		if metadata.ClearDifficulty {
			value = ""
		}
		request.Difficulty = &value
	}
	if metadata.ClearTags || command.Flags().Changed("tag") {
		values := append([]string(nil), metadata.Tags...)
		if metadata.ClearTags {
			values = []string{}
		}
		request.Tags = &values
	}
	if command.Flags().Changed("subtasks-json") {
		var values []apiclient.TaskSubtaskInput
		if err := json.Unmarshal([]byte(metadata.SubtasksJSON), &values); err != nil {
			return fmt.Errorf("parse --subtasks-json: %w", err)
		}
		request.Subtasks = &values
	} else if metadata.ClearSubtasks || command.Flags().Changed("subtask") {
		values := make([]apiclient.TaskSubtaskInput, 0, len(metadata.Subtasks))
		if !metadata.ClearSubtasks {
			for position, title := range metadata.Subtasks {
				values = append(values, apiclient.TaskSubtaskInput{
					Title: title, Position: position,
				})
			}
		}
		request.Subtasks = &values
	}
	return nil
}
