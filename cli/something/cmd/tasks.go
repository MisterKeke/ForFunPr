package cmd

import (
	"context"
	"fmt"

	"currency-wails/cli/internal/apiclient"

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
		newTaskCreateCommand(dependencies),
		newTaskUpdateCommand(dependencies),
		newTaskToggleCommand(dependencies),
		newTaskDeleteCommand(dependencies),
	)
	return command
}

func newTaskListCommand(dependencies commandDependencies) *cobra.Command {
	var date string

	command := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.ListTasks(command.Context(), date)
			if err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(
		&date,
		"date",
		"",
		"only list tasks due on YYYY-MM-DD",
	)
	return command
}

func newTaskTodayCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "List today's incomplete tasks",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.TodayTasks(command.Context())
			if err != nil {
				return fmt.Errorf("list today's tasks: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
}

func newTaskCreateCommand(dependencies commandDependencies) *cobra.Command {
	var request apiclient.TaskWriteRequest

	command := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Title", &request.Title); err != nil {
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
	addTaskWriteFlags(command, &request)
	return command
}

func newTaskUpdateCommand(dependencies commandDependencies) *cobra.Command {
	var idText string
	var request apiclient.TaskWriteRequest

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
	addTaskWriteFlags(command, &request)
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
		) ([]apiclient.Task, error) {
			return client.ToggleTask(ctx, id)
		},
	)
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
		) ([]apiclient.Task, error) {
			return client.DeleteTask(ctx, id)
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
	) ([]apiclient.Task, error),
) *cobra.Command {
	var idText string

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

			result, err := call(client, command.Context(), id)
			if err != nil {
				return fmt.Errorf("%s task: %w", use, err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "task ID")
	return command
}

func addTaskWriteFlags(
	command *cobra.Command,
	request *apiclient.TaskWriteRequest,
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
}
