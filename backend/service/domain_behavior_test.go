package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNoteOptimisticConcurrencyStateAndFiltering(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	created, err := service.CreateNoteContext(ctx, NoteCreateRequest{
		Title: "  Release notes  ", Body: "first body", Pinned: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Title != "Release notes" || created.Revision != 1 {
		t.Fatalf("created note = %#v", created)
	}

	unchanged, err := service.SetNotePinnedContext(ctx, NoteStateRequest{
		ID: created.ID, Value: false, ExpectedRevision: 999,
	})
	if err != nil || unchanged.Revision != created.Revision {
		t.Fatalf("idempotent state change = %#v, %v", unchanged, err)
	}
	pinned, err := service.SetNotePinnedContext(ctx, NoteStateRequest{
		ID: created.ID, Value: true, ExpectedRevision: created.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !pinned.Pinned || pinned.Revision != created.Revision+1 {
		t.Fatalf("pinned note = %#v", pinned)
	}

	_, err = service.UpdateNoteContext(ctx, NoteUpdateRequest{
		ID: created.ID, Title: "stale", Body: "stale", ExpectedRevision: created.Revision,
	})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("stale update error = %v, want ConflictError", err)
	}

	archived, err := service.SetNoteArchivedContext(ctx, NoteStateRequest{
		ID: created.ID, Value: true, ExpectedRevision: pinned.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	active, err := service.ListNotesContext(ctx, NoteListFilter{ArchiveStatus: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if active.Total != 0 {
		t.Fatalf("active notes = %#v", active)
	}
	archivedOnly, err := service.ListNotesContext(ctx, NoteListFilter{
		ArchiveStatus: "archived", Query: "release", Pinned: boolTestPointer(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if archivedOnly.Total != 1 || archivedOnly.Notes[0].ID != archived.ID {
		t.Fatalf("archived notes = %#v", archivedOnly)
	}

	if _, err := service.CreateNoteContext(ctx, NoteCreateRequest{
		Title: "too large", Body: strings.Repeat("x", maximumNoteBodyBytes+1),
	}); err == nil {
		t.Fatal("oversized note body unexpectedly accepted")
	}
	if err := service.DeleteNoteContext(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetNoteContext(ctx, created.ID); err == nil {
		t.Fatal("deleted note still exists")
	}
}

func boolTestPointer(value bool) *bool { return &value }

func TestBookmarkNormalizationConcurrencyTagsAndLiteralSearch(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	display, normalized, err := NormalizeBookmarkURL(" HTTPS://Example.COM:443/a?q=1 ")
	if err != nil {
		t.Fatal(err)
	}
	if display != "https://example.com/a?q=1" || normalized != display {
		t.Fatalf("normalized bookmark URL = (%q, %q)", display, normalized)
	}
	if _, _, err := NormalizeBookmarkURL("javascript:alert(1)"); err == nil {
		t.Fatal("unsafe bookmark scheme unexpectedly accepted")
	}

	created, err := service.CreateBookmarkContext(ctx, BookmarkCreateRequest{
		URL: "https://example.com:443/a?q=1", Title: "100%_Useful",
		Tags: []string{"Go", "go", "Testing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Tags) != 2 || created.Revision != 1 {
		t.Fatalf("created bookmark = %#v", created)
	}
	if _, err := service.CreateBookmarkContext(ctx, BookmarkCreateRequest{
		URL: "HTTPS://EXAMPLE.COM/a?q=1", Title: "duplicate",
	}); err == nil {
		t.Fatal("normalized duplicate URL unexpectedly accepted")
	}

	unchanged, err := service.SetBookmarkReadContext(ctx, BookmarkReadRequest{
		ID: created.ID, Read: false, ExpectedRevision: 999,
	})
	if err != nil || unchanged.Revision != created.Revision {
		t.Fatalf("idempotent read state = %#v, %v", unchanged, err)
	}
	read, err := service.SetBookmarkReadContext(ctx, BookmarkReadRequest{
		ID: created.ID, Read: true, ExpectedRevision: created.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !read.Read || read.ReadAt == "" || read.Revision != created.Revision+1 {
		t.Fatalf("read bookmark = %#v", read)
	}

	_, err = service.UpdateBookmarkContext(ctx, BookmarkUpdateRequest{
		ID: created.ID, URL: created.URL, Title: "stale", ExpectedRevision: created.Revision,
	})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("stale bookmark update error = %v, want ConflictError", err)
	}
	updated, err := service.UpdateBookmarkContext(ctx, BookmarkUpdateRequest{
		ID: created.ID, URL: created.URL, Title: "updated 100%_Useful", Tags: []string{"Testing"},
		ExpectedRevision: read.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "Testing" {
		t.Fatalf("updated tags = %#v", updated.Tags)
	}
	var orphanCount int
	if err := service.db.QueryRow(`SELECT COUNT(*) FROM bookmark_tags WHERE name_normalized = 'go'`).Scan(&orphanCount); err != nil {
		t.Fatal(err)
	}
	if orphanCount != 0 {
		t.Fatal("unused bookmark tag was not removed")
	}

	for _, query := range []string{"%", "_"} {
		result, err := service.ListBookmarksContext(ctx, BookmarkFilter{Query: query})
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != 1 || result.Bookmarks[0].ID != created.ID {
			t.Fatalf("literal search %q returned %#v", query, result)
		}
	}
}

func TestNoteTopicGraphAndTodoLinksEnforceRelationships(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	noteOne, err := service.CreateNoteContext(ctx, NoteCreateRequest{Title: "One", Body: "First"})
	if err != nil {
		t.Fatal(err)
	}
	noteTwo, err := service.CreateNoteContext(ctx, NoteCreateRequest{Title: "Two", Body: "Second"})
	if err != nil {
		t.Fatal(err)
	}
	noteThree, err := service.CreateNoteContext(ctx, NoteCreateRequest{Title: "Three"})
	if err != nil {
		t.Fatal(err)
	}
	topicOne, err := service.CreateNoteTopicContext(ctx, NoteTopicWriteRequest{Title: "Topic one"})
	if err != nil {
		t.Fatal(err)
	}
	topicTwo, err := service.CreateNoteTopicContext(ctx, NoteTopicWriteRequest{Title: "Topic two"})
	if err != nil {
		t.Fatal(err)
	}
	blockOne, err := service.AddNoteTopicBlockContext(ctx, NoteTopicBlockCreateRequest{
		TopicID: topicOne.ID, NoteID: noteOne.ID, PositionX: 10.04, PositionY: 20.06,
	})
	if err != nil {
		t.Fatal(err)
	}
	if blockOne.PositionX != 10 || blockOne.PositionY != 20.1 {
		t.Fatalf("rounded block position = (%v, %v)", blockOne.PositionX, blockOne.PositionY)
	}
	blockTwo, err := service.AddNoteTopicBlockContext(ctx, NoteTopicBlockCreateRequest{
		TopicID: topicOne.ID, NoteID: noteTwo.ID, PositionX: 30, PositionY: 40,
	})
	if err != nil {
		t.Fatal(err)
	}
	foreignBlock, err := service.AddNoteTopicBlockContext(ctx, NoteTopicBlockCreateRequest{
		TopicID: topicTwo.ID, NoteID: noteThree.ID, PositionX: 1, PositionY: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddNoteTopicBlockContext(ctx, NoteTopicBlockCreateRequest{
		TopicID: topicOne.ID, NoteID: noteOne.ID, PositionX: 0, PositionY: 0,
	}); err == nil {
		t.Fatal("duplicate topic block unexpectedly accepted")
	}
	if _, err := service.CreateNoteTopicConnectionContext(ctx, NoteTopicConnectionCreateRequest{
		TopicID: topicOne.ID, FromBlockID: blockOne.ID, ToBlockID: blockOne.ID,
	}); err == nil {
		t.Fatal("self connection unexpectedly accepted")
	}
	connection, err := service.CreateNoteTopicConnectionContext(ctx, NoteTopicConnectionCreateRequest{
		TopicID: topicOne.ID, FromBlockID: blockOne.ID, ToBlockID: blockTwo.ID, RelationType: "related",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateNoteTopicConnectionContext(ctx, NoteTopicConnectionCreateRequest{
		TopicID: topicOne.ID, FromBlockID: blockTwo.ID, ToBlockID: blockOne.ID, RelationType: "related",
	}); err == nil {
		t.Fatal("reversed duplicate related connection unexpectedly accepted")
	}
	if _, err := service.CreateNoteTopicConnectionContext(ctx, NoteTopicConnectionCreateRequest{
		TopicID: topicOne.ID, FromBlockID: blockOne.ID, ToBlockID: foreignBlock.ID,
	}); err == nil {
		t.Fatal("cross-topic connection unexpectedly accepted")
	}

	todos, err := service.CreateTodoContext(ctx, TodoCreateRequest{Title: "Linked task", Priority: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	todoID := todos[0].ID
	link := NoteTodoConnectionRequest{NoteID: noteOne.ID, TodoID: todoID}
	if err := service.LinkNoteTodoContext(ctx, link); err != nil {
		t.Fatal(err)
	}
	if err := service.LinkNoteTodoContext(ctx, link); err != nil {
		t.Fatalf("idempotent link failed: %v", err)
	}
	linked, err := service.ListNoteTodosContext(ctx, noteOne.ID)
	if err != nil || len(linked) != 1 || linked[0].ID != todoID {
		t.Fatalf("linked tasks = %#v, %v", linked, err)
	}

	if err := service.DeleteNoteTopicBlockContext(ctx, blockOne.ID); err != nil {
		t.Fatal(err)
	}
	board, err := service.GetNoteTopicBoardContext(ctx, topicOne.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Connections) != 0 || len(board.Blocks) != 1 || connection.ID <= 0 {
		t.Fatalf("board after cascade = %#v", board)
	}
}

func TestTodoDatesCompletionAndDeletion(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	if normalized, err := NormalizeDate(" 2028-02-29 ", true); err != nil || normalized != "2028-02-29" {
		t.Fatalf("leap date = %q, %v", normalized, err)
	}
	for _, value := range []string{"2027-02-29", "2026-13-01", "01-01-2026"} {
		if _, err := NormalizeDate(value, true); err == nil {
			t.Fatalf("invalid date %q accepted", value)
		}
	}

	today := time.Now().Format("2006-01-02")
	todos, err := service.CreateTodoContext(ctx, TodoCreateRequest{
		Title: "Today", DueDate: today, Priority: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	id := todos[0].ID
	todayTodos, err := service.GetTodayIncompleteTodos()
	if err != nil || len(todayTodos) != 1 || todayTodos[0].ID != id {
		t.Fatalf("today todos = %#v, %v", todayTodos, err)
	}
	toggled, err := service.ToggleTodoContext(ctx, TodoIDRequest{ID: id})
	if err != nil {
		t.Fatal(err)
	}
	if len(toggled) != 1 || !toggled[0].Done {
		t.Fatalf("toggled todos = %#v", toggled)
	}
	todayTodos, err = service.GetTodayIncompleteTodos()
	if err != nil || len(todayTodos) != 0 {
		t.Fatalf("completed task remained in today list: %#v, %v", todayTodos, err)
	}
	remaining, err := service.DeleteTodoContext(ctx, TodoIDRequest{ID: id})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("delete result = %#v, %v", remaining, err)
	}
	if _, err := service.DeleteTodoContext(ctx, TodoIDRequest{ID: id}); err == nil {
		t.Fatal("deleting a missing todo unexpectedly succeeded")
	}
}

func TestSetupCRUDPreservesAppOrderAndRejectsConflicts(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	for _, values := range [][2]string{{"Editor", `C:\\Apps\\editor.exe`}, {"Terminal", `C:\\Apps\\terminal.exe`}} {
		if _, err := service.db.ExecContext(ctx,
			`INSERT INTO desktop_apps(display_name, executable_path) VALUES (?, ?)`, values[0], values[1]); err != nil {
			t.Fatal(err)
		}
	}
	apps, err := service.ListDesktopAppsContext(ctx)
	if err != nil || len(apps) != 2 {
		t.Fatalf("apps = %#v, %v", apps, err)
	}
	created, err := service.CreateSetupContext(ctx, SetupCreateRequest{
		Name: " Work ", Description: " daily ", AppIDs: []int{apps[1].ID, apps[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Work" || len(created.AppIDs) != 2 || created.AppIDs[0] != apps[1].ID {
		t.Fatalf("created setup = %#v", created)
	}
	if _, err := service.CreateSetupContext(ctx, SetupCreateRequest{
		Name: "work", AppIDs: []int{apps[0].ID},
	}); err == nil {
		t.Fatal("case-insensitive duplicate setup name accepted")
	}
	if _, err := service.UpdateSetupContext(ctx, SetupUpdateRequest{
		ID: created.ID, Name: "Work", AppIDs: []int{apps[0].ID, apps[0].ID},
	}); err == nil {
		t.Fatal("duplicate application in setup accepted")
	}
	updated, err := service.UpdateSetupContext(ctx, SetupUpdateRequest{
		ID: created.ID, Name: "Focused", AppIDs: []int{apps[0].ID},
	})
	if err != nil || updated.Name != "Focused" || len(updated.AppIDs) != 1 {
		t.Fatalf("updated setup = %#v, %v", updated, err)
	}
	if err := service.DeleteSetupContext(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetSetupContext(ctx, created.ID); err == nil {
		t.Fatal("deleted setup still exists")
	}
}

func TestClipboardDeduplicationFilteringPruningAndPinnedClear(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	settings := ClipboardSettings{
		CollectionEnabled: true, RetentionDays: 30, MaximumItems: 10, MaximumTextBytes: 4096,
	}
	if _, err := service.UpdateClipboardSettingsContext(ctx, settings); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"https://example.com/path", "plain text", "plain text"} {
		if err := service.RecordClipboardTextContext(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	items, err := service.ListClipboardItemsContext(ctx, ClipboardListFilter{Query: "plain", Kind: "text"})
	if err != nil || items.Total != 1 || items.Items[0].CopyCount != 2 {
		t.Fatalf("clipboard items = %#v, %v", items, err)
	}
	if clipboardKind("https://example.com") != "url" || clipboardKind("not a url") != "text" {
		t.Fatal("clipboard kind classification is incorrect")
	}
	if err := service.SetClipboardItemPinnedContext(ctx, items.Items[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := service.ClearClipboardHistoryContext(ctx, true); err != nil {
		t.Fatal(err)
	}
	remaining, err := service.ListClipboardItemsContext(ctx, ClipboardListFilter{})
	if err != nil || remaining.Total != 1 || !remaining.Items[0].Pinned {
		t.Fatalf("clipboard after clear = %#v, %v", remaining, err)
	}

	if _, err := service.db.ExecContext(ctx, `UPDATE clipboard_items SET is_pinned = 0,
		last_copied_at = '2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if err := service.PruneClipboardHistoryContext(ctx, settings); err != nil {
		t.Fatal(err)
	}
	remaining, err = service.ListClipboardItemsContext(ctx, ClipboardListFilter{})
	if err != nil || remaining.Total != 0 {
		t.Fatalf("expired clipboard items = %#v, %v", remaining, err)
	}
}
