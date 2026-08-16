package cmd

import (
	"fmt"
	"os"
	"strings"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newNotesCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{Use: "notes", Short: "Manage local notes", Args: cobra.NoArgs}
	command.AddCommand(
		newNoteListCommand(dependencies),
		newNoteGetCommand(dependencies),
		newNoteCreateCommand(dependencies),
		newNoteUpdateCommand(dependencies),
		newNoteStateCommand(dependencies, "pin", "Pin a note", true, true),
		newNoteStateCommand(dependencies, "unpin", "Unpin a note", true, false),
		newNoteStateCommand(dependencies, "archive", "Archive a note", false, true),
		newNoteStateCommand(dependencies, "restore", "Restore an archived note", false, false),
		newNoteDeleteCommand(dependencies),
	)
	return command
}

func newNoteListCommand(dependencies commandDependencies) *cobra.Command {
	var filter apiclient.NoteListFilter
	var pinned bool
	var unpinned bool
	filter.Archive = "active"
	command := &cobra.Command{
		Use: "list", Short: "List notes", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if pinned && unpinned {
				return fmt.Errorf("--pinned and --unpinned cannot be used together")
			}
			if pinned || unpinned {
				value := pinned
				filter.Pinned = &value
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.ListNotes(command.Context(), filter)
			if err != nil {
				return fmt.Errorf("list notes: %w", err)
			}
			if strings.EqualFold(dependencies.outputFormat(), "json") {
				return dependencies.writeValue(command, result)
			}
			return dependencies.writeValue(command, result.Notes)
		},
	}
	command.Flags().StringVar(&filter.Query, "search", "", "search note titles and bodies")
	command.Flags().StringVar(&filter.Archive, "archive", "active", "active, archived, or all")
	command.Flags().BoolVar(&pinned, "pinned", false, "only list pinned notes")
	command.Flags().BoolVar(&unpinned, "unpinned", false, "only list unpinned notes")
	command.Flags().IntVar(&filter.Limit, "limit", 50, "maximum notes to return")
	command.Flags().IntVar(&filter.Offset, "offset", 0, "notes to skip")
	return command
}

func newNoteGetCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{
		Use: "get [NOTE_ID]", Short: "Get a note", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			id, err := parsePositiveCLIInteger("Note ID", arguments[0])
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			note, err := client.GetNote(command.Context(), id)
			if err != nil {
				return fmt.Errorf("get note: %w", err)
			}
			return dependencies.writeValue(command, note)
		},
	}
}

func newNoteCreateCommand(dependencies commandDependencies) *cobra.Command {
	var request apiclient.NoteCreateRequest
	var bodyFile string
	command := &cobra.Command{
		Use: "create", Short: "Create a note", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if bodyFile != "" {
				if command.Flags().Changed("body") {
					return fmt.Errorf("--body and --body-file cannot be used together")
				}
				contents, err := os.ReadFile(bodyFile)
				if err != nil {
					return fmt.Errorf("read note body file: %w", err)
				}
				request.Body = string(contents)
			}
			if strings.TrimSpace(request.Title) == "" && strings.TrimSpace(request.Body) == "" {
				if err := newPrompter(command).required("Title", &request.Title); err != nil {
					return err
				}
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			note, err := client.CreateNote(command.Context(), request)
			if err != nil {
				return fmt.Errorf("create note: %w", err)
			}
			return dependencies.writeValue(command, note)
		},
	}
	command.Flags().StringVar(&request.Title, "title", "", "note title")
	command.Flags().StringVar(&request.Body, "body", "", "note body")
	command.Flags().StringVar(&bodyFile, "body-file", "", "read note body from a file")
	command.Flags().BoolVar(&request.Pinned, "pin", false, "pin the new note")
	return command
}

func newNoteUpdateCommand(dependencies commandDependencies) *cobra.Command {
	var title string
	var body string
	var bodyFile string
	command := &cobra.Command{
		Use: "update [NOTE_ID]", Short: "Update a note", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			id, err := parsePositiveCLIInteger("Note ID", arguments[0])
			if err != nil {
				return err
			}
			if bodyFile != "" && command.Flags().Changed("body") {
				return fmt.Errorf("--body and --body-file cannot be used together")
			}
			if !command.Flags().Changed("title") && !command.Flags().Changed("body") && bodyFile == "" {
				return fmt.Errorf("set --title, --body, or --body-file")
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			current, err := client.GetNote(command.Context(), id)
			if err != nil {
				return fmt.Errorf("load note before update: %w", err)
			}
			if command.Flags().Changed("title") {
				current.Title = title
			}
			if command.Flags().Changed("body") {
				current.Body = body
			}
			if bodyFile != "" {
				contents, err := os.ReadFile(bodyFile)
				if err != nil {
					return fmt.Errorf("read note body file: %w", err)
				}
				current.Body = string(contents)
			}
			note, err := client.UpdateNote(command.Context(), id, apiclient.NoteUpdateRequest{
				Title: current.Title, Body: current.Body, ExpectedRevision: current.Revision,
			})
			if err != nil {
				return fmt.Errorf("update note: %w", err)
			}
			return dependencies.writeValue(command, note)
		},
	}
	command.Flags().StringVar(&title, "title", "", "replacement note title")
	command.Flags().StringVar(&body, "body", "", "replacement note body")
	command.Flags().StringVar(&bodyFile, "body-file", "", "read replacement body from a file")
	return command
}

func newNoteStateCommand(
	dependencies commandDependencies,
	use string,
	short string,
	pinned bool,
	value bool,
) *cobra.Command {
	return &cobra.Command{
		Use: use + " [NOTE_ID]", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			id, err := parsePositiveCLIInteger("Note ID", arguments[0])
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			current, err := client.GetNote(command.Context(), id)
			if err != nil {
				return fmt.Errorf("load note before %s: %w", use, err)
			}
			request := apiclient.NoteStateRequest{Value: value, ExpectedRevision: current.Revision}
			var note apiclient.Note
			if pinned {
				note, err = client.SetNotePinned(command.Context(), id, request)
			} else {
				note, err = client.SetNoteArchived(command.Context(), id, request)
			}
			if err != nil {
				return fmt.Errorf("%s note: %w", use, err)
			}
			return dependencies.writeValue(command, note)
		},
	}
}

func newNoteDeleteCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{
		Use: "delete [NOTE_ID]", Short: "Permanently delete a note", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			id, err := parsePositiveCLIInteger("Note ID", arguments[0])
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			if err := client.DeleteNote(command.Context(), id); err != nil {
				return fmt.Errorf("delete note: %w", err)
			}
			return dependencies.writeSuccess(command, "Note deleted.")
		},
	}
}
