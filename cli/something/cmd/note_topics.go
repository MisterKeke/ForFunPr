package cmd

import (
	"encoding/json"
	"fmt"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newNoteTopicsCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{Use: "topics", Short: "Manage note topic boards", Args: cobra.NoArgs}
	command.AddCommand(
		newNoteTopicListCommand(dependencies), newNoteTopicGetCommand(dependencies),
		newNoteTopicCreateCommand(dependencies), newNoteTopicUpdateCommand(dependencies),
		newNoteTopicDeleteCommand(dependencies), newNoteTopicPickerCommand(dependencies),
		newNoteTopicAddBlockCommand(dependencies), newNoteTopicMoveBlocksCommand(dependencies),
		newNoteTopicDeleteBlockCommand(dependencies), newNoteTopicConnectCommand(dependencies),
		newNoteTopicDisconnectCommand(dependencies),
	)
	return command
}

func newNoteTopicListCommand(dependencies commandDependencies) *cobra.Command {
	var limit, offset int
	command := &cobra.Command{Use: "list", Short: "List note topics", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.ListNoteTopics(command.Context(), limit, offset)
		if err != nil {
			return fmt.Errorf("list note topics: %w", err)
		}
		if dependencies.outputFormat() == "json" {
			return dependencies.writeValue(command, result)
		}
		return dependencies.writeValue(command, result.Items)
	}}
	command.Flags().IntVar(&limit, "limit", 50, "maximum topics to return")
	command.Flags().IntVar(&offset, "offset", 0, "topics to skip")
	return command
}

func newNoteTopicGetCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{Use: "get [TOPIC_ID]", Short: "Get a topic board", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := parsePositiveCLIInteger("Topic ID", args[0])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.GetNoteTopicBoard(command.Context(), id)
		if err != nil {
			return fmt.Errorf("get note topic board: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
}

func newNoteTopicCreateCommand(dependencies commandDependencies) *cobra.Command {
	var title string
	command := &cobra.Command{Use: "create", Short: "Create a note topic", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.CreateNoteTopic(command.Context(), apiclient.NoteTopicWriteRequest{Title: title})
		if err != nil {
			return fmt.Errorf("create note topic: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().StringVar(&title, "title", "", "topic title")
	_ = command.MarkFlagRequired("title")
	return command
}

func newNoteTopicUpdateCommand(dependencies commandDependencies) *cobra.Command {
	var title string
	var revision int
	command := &cobra.Command{Use: "update [TOPIC_ID]", Short: "Update a note topic", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := parsePositiveCLIInteger("Topic ID", args[0])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.UpdateNoteTopic(command.Context(), id, apiclient.NoteTopicWriteRequest{Title: title, ExpectedRevision: optionalRevision(command, revision)})
		if err != nil {
			return fmt.Errorf("update note topic: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().StringVar(&title, "title", "", "replacement title")
	command.Flags().IntVar(&revision, "expected-revision", 0, "reject the update if the topic revision changed")
	_ = command.MarkFlagRequired("title")
	return command
}

func newNoteTopicDeleteCommand(dependencies commandDependencies) *cobra.Command {
	return newNoteTopicRevisionDeleteCommand(dependencies, "delete [TOPIC_ID]", "Delete a note topic", func(client *apiclient.Client, command *cobra.Command, id int, revision *int) (any, error) {
		return client.DeleteNoteTopic(command.Context(), id, revision)
	})
}

func newNoteTopicDeleteBlockCommand(dependencies commandDependencies) *cobra.Command {
	return newNoteTopicRevisionDeleteCommand(dependencies, "delete-block [BLOCK_ID]", "Delete a topic block", func(client *apiclient.Client, command *cobra.Command, id int, revision *int) (any, error) {
		return client.DeleteNoteTopicBlock(command.Context(), id, revision)
	})
}

func newNoteTopicDisconnectCommand(dependencies commandDependencies) *cobra.Command {
	return newNoteTopicRevisionDeleteCommand(dependencies, "disconnect [CONNECTION_ID]", "Delete a topic connection", func(client *apiclient.Client, command *cobra.Command, id int, revision *int) (any, error) {
		return client.DeleteNoteTopicConnection(command.Context(), id, revision)
	})
}

func newNoteTopicRevisionDeleteCommand(dependencies commandDependencies, use, short string, action func(*apiclient.Client, *cobra.Command, int, *int) (any, error)) *cobra.Command {
	var revision int
	command := &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := parsePositiveCLIInteger("ID", args[0])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := action(client, command, id, optionalRevision(command, revision))
		if err != nil {
			return err
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().IntVar(&revision, "expected-revision", 0, "reject the deletion if the topic revision changed")
	return command
}

func newNoteTopicPickerCommand(dependencies commandDependencies) *cobra.Command {
	var search string
	var limit, offset int
	command := &cobra.Command{Use: "picker [TOPIC_ID]", Short: "Search notes not yet on a topic", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := parsePositiveCLIInteger("Topic ID", args[0])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.SearchNoteTopicPicker(command.Context(), id, search, limit, offset)
		if err != nil {
			return fmt.Errorf("search topic note picker: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().StringVar(&search, "search", "", "search note titles and bodies")
	command.Flags().IntVar(&limit, "limit", 50, "maximum notes to return")
	command.Flags().IntVar(&offset, "offset", 0, "notes to skip")
	return command
}

func newNoteTopicAddBlockCommand(dependencies commandDependencies) *cobra.Command {
	var noteID, revision int
	var x, y float64
	command := &cobra.Command{Use: "add-block [TOPIC_ID]", Short: "Add a note to a topic", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		topicID, err := parsePositiveCLIInteger("Topic ID", args[0])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.AddNoteTopicBlock(command.Context(), topicID, apiclient.NoteTopicBlockCreateRequest{NoteID: noteID, PositionX: x, PositionY: y, ExpectedRevision: optionalRevision(command, revision)})
		if err != nil {
			return fmt.Errorf("add topic block: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().IntVar(&noteID, "note-id", 0, "note ID")
	command.Flags().Float64Var(&x, "x", 0, "horizontal board position")
	command.Flags().Float64Var(&y, "y", 0, "vertical board position")
	command.Flags().IntVar(&revision, "expected-revision", 0, "reject if the topic revision changed")
	_ = command.MarkFlagRequired("note-id")
	return command
}

func newNoteTopicMoveBlocksCommand(dependencies commandDependencies) *cobra.Command {
	var positionsJSON string
	var revision int
	command := &cobra.Command{Use: "move-blocks [TOPIC_ID]", Short: "Atomically update topic block positions", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		topicID, err := parsePositiveCLIInteger("Topic ID", args[0])
		if err != nil {
			return err
		}
		var positions []apiclient.NoteTopicBlockPosition
		if err := json.Unmarshal([]byte(positionsJSON), &positions); err != nil {
			return fmt.Errorf("parse --positions JSON: %w", err)
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.UpdateNoteTopicBlockPositions(command.Context(), topicID, apiclient.NoteTopicBlockPositionsRequest{Positions: positions, ExpectedRevision: optionalRevision(command, revision)})
		if err != nil {
			return fmt.Errorf("move topic blocks: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().StringVar(&positionsJSON, "positions", "", "JSON array of block_id, position_x, and position_y objects")
	command.Flags().IntVar(&revision, "expected-revision", 0, "reject if the topic revision changed")
	_ = command.MarkFlagRequired("positions")
	return command
}

func newNoteTopicConnectCommand(dependencies commandDependencies) *cobra.Command {
	var fromID, toID, revision int
	var relation string
	command := &cobra.Command{Use: "connect [TOPIC_ID]", Short: "Connect two topic blocks", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		topicID, err := parsePositiveCLIInteger("Topic ID", args[0])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.CreateNoteTopicConnection(command.Context(), topicID, apiclient.NoteTopicConnectionCreateRequest{FromBlockID: fromID, ToBlockID: toID, RelationType: relation, ExpectedRevision: optionalRevision(command, revision)})
		if err != nil {
			return fmt.Errorf("connect topic blocks: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().IntVar(&fromID, "from-block-id", 0, "source block ID")
	command.Flags().IntVar(&toID, "to-block-id", 0, "destination block ID")
	command.Flags().StringVar(&relation, "relation", "leads_to", "leads_to or related")
	command.Flags().IntVar(&revision, "expected-revision", 0, "reject if the topic revision changed")
	_ = command.MarkFlagRequired("from-block-id")
	_ = command.MarkFlagRequired("to-block-id")
	return command
}

func newNoteTasksCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{Use: "tasks", Short: "Manage note-task links", Args: cobra.NoArgs}
	command.AddCommand(newNoteTaskListCommand(dependencies), newTaskNotesListCommand(dependencies), newNoteTaskLinkCommand(dependencies, false), newNoteTaskLinkCommand(dependencies, true))
	return command
}

func newNoteTaskListCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{Use: "list [NOTE_ID]", Short: "List tasks linked to a note", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := parsePositiveCLIInteger("Note ID", args[0])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.ListNoteTodos(command.Context(), id)
		if err != nil {
			return fmt.Errorf("list note tasks: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
}

func newTaskNotesListCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{Use: "notes [TASK_ID]", Short: "List notes linked to a task", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := parsePositiveCLIInteger("Task ID", args[0])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.ListTodoNotes(command.Context(), id)
		if err != nil {
			return fmt.Errorf("list task notes: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
}

func newNoteTaskLinkCommand(dependencies commandDependencies, unlink bool) *cobra.Command {
	use, short := "link [NOTE_ID] [TASK_ID]", "Link a task to a note"
	if unlink {
		use, short = "unlink [NOTE_ID] [TASK_ID]", "Unlink a task from a note"
	}
	return &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		noteID, err := parsePositiveCLIInteger("Note ID", args[0])
		if err != nil {
			return err
		}
		todoID, err := parsePositiveCLIInteger("Task ID", args[1])
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		var result apiclient.NoteTodoMutationResult
		if unlink {
			result, err = client.UnlinkNoteTodo(command.Context(), noteID, todoID)
		} else {
			result, err = client.LinkNoteTodo(command.Context(), noteID, todoID)
		}
		if err != nil {
			return err
		}
		return dependencies.writeValue(command, result)
	}}
}

func optionalRevision(command *cobra.Command, revision int) *int {
	if !command.Flags().Changed("expected-revision") {
		return nil
	}
	return &revision
}
