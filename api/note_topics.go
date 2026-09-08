package api

import (
	"net/http"
	"strconv"
	"strings"

	backend "something/backend/service"
)

type noteTopicWriteRequest struct {
	Title            string `json:"title"`
	ExpectedRevision *int   `json:"expected_revision,omitempty"`
}

type noteTopicBlockWriteRequest struct {
	NoteID           int     `json:"note_id"`
	PositionX        float64 `json:"position_x"`
	PositionY        float64 `json:"position_y"`
	ExpectedRevision *int    `json:"expected_revision,omitempty"`
}

type noteTopicPositionsWriteRequest struct {
	Positions        []backend.NoteTopicBlockPositionRequest `json:"positions"`
	ExpectedRevision *int                                    `json:"expected_revision,omitempty"`
}

type noteTopicConnectionWriteRequest struct {
	FromBlockID      int    `json:"from_block_id"`
	ToBlockID        int    `json:"to_block_id"`
	RelationType     string `json:"relation_type"`
	ExpectedRevision *int   `json:"expected_revision,omitempty"`
}

type noteTodoWriteRequest struct {
	TodoID int `json:"todo_id"`
}

func noteTopicsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseNonNegativeQueryInteger(w, r, "limit")
		if !ok {
			return
		}
		offset, ok := parseNonNegativeQueryInteger(w, r, "offset")
		if !ok {
			return
		}
		result, err := app.ListNoteTopicsPageContext(r.Context(), backend.NoteTopicListFilter{Limit: limit, Offset: offset})
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topics_failed", "Note topics could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func noteTopicHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic")
		if !ok {
			return
		}
		topic, err := app.GetNoteTopicContext(r.Context(), id)
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_failed", "Note topic could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, topic)
	}
}

func createNoteTopicHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request noteTopicWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		topic, err := app.CreateNoteTopicContext(r.Context(), backend.NoteTopicWriteRequest{Title: request.Title})
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_create_failed", "Note topic could not be created.")
			return
		}
		writeJSON(w, http.StatusCreated, topic)
	}
}

func updateNoteTopicHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic")
		if !ok {
			return
		}
		var request noteTopicWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		topic, err := app.RenameNoteTopicContext(r.Context(), backend.NoteTopicWriteRequest{
			ID: id, Title: request.Title, ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_update_failed", "Note topic could not be updated.")
			return
		}
		writeJSON(w, http.StatusOK, topic)
	}
}

func deleteNoteTopicHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic")
		if !ok {
			return
		}
		expected, ok := parseOptionalRevision(w, r)
		if !ok {
			return
		}
		result, err := app.DeleteNoteTopicWithRevisionContext(r.Context(), backend.NoteTopicIDRequest{ID: id, ExpectedRevision: expected})
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_delete_failed", "Note topic could not be deleted.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func noteTopicBoardHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic")
		if !ok {
			return
		}
		board, err := app.GetNoteTopicBoardContext(r.Context(), id)
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_board_failed", "Note topic board could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, board)
	}
}

func noteTopicPickerHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic")
		if !ok {
			return
		}
		limit, ok := parseNonNegativeQueryInteger(w, r, "limit")
		if !ok {
			return
		}
		offset, ok := parseNonNegativeQueryInteger(w, r, "offset")
		if !ok {
			return
		}
		result, err := app.SearchNoteTopicPickerContext(r.Context(), backend.NoteTopicPickerFilter{
			TopicID: id, Query: strings.TrimSpace(r.URL.Query().Get("q")), Limit: limit, Offset: offset,
		})
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_picker_failed", "Notes could not be searched.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func addNoteTopicBlockHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic")
		if !ok {
			return
		}
		var request noteTopicBlockWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		block, err := app.AddNoteTopicBlockContext(r.Context(), backend.NoteTopicBlockCreateRequest{
			TopicID: id, NoteID: request.NoteID, PositionX: request.PositionX,
			PositionY: request.PositionY, ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_block_create_failed", "Note could not be added to the topic.")
			return
		}
		writeJSON(w, http.StatusCreated, block)
	}
}

func updateNoteTopicBlockPositionsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic")
		if !ok {
			return
		}
		var request noteTopicPositionsWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		result, err := app.UpdateNoteTopicBlockPositionsContext(r.Context(), backend.NoteTopicBlockPositionsRequest{
			TopicID: id, Positions: request.Positions, ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_positions_failed", "Block positions could not be updated.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func deleteNoteTopicBlockHandler(app *backend.Service) http.HandlerFunc {
	return deleteNoteTopicChildHandler(app, true)
}

func deleteNoteTopicConnectionHandler(app *backend.Service) http.HandlerFunc {
	return deleteNoteTopicChildHandler(app, false)
}

func deleteNoteTopicChildHandler(app *backend.Service, block bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic item")
		if !ok {
			return
		}
		expected, ok := parseOptionalRevision(w, r)
		if !ok {
			return
		}
		request := backend.NoteTopicIDRequest{ID: id, ExpectedRevision: expected}
		var result backend.NoteTopicMutationResult
		var err error
		if block {
			result, err = app.DeleteNoteTopicBlockWithRevisionContext(r.Context(), request)
		} else {
			result, err = app.DeleteNoteTopicConnectionWithRevisionContext(r.Context(), request)
		}
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_delete_failed", "The topic item could not be deleted.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func createNoteTopicConnectionHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note topic")
		if !ok {
			return
		}
		var request noteTopicConnectionWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		connection, err := app.CreateNoteTopicConnectionContext(r.Context(), backend.NoteTopicConnectionCreateRequest{
			TopicID: id, FromBlockID: request.FromBlockID, ToBlockID: request.ToBlockID,
			RelationType: request.RelationType, ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeOrganizerError(w, err, "note_topic", "note_topic_connection_create_failed", "Connection could not be created.")
			return
		}
		writeJSON(w, http.StatusCreated, connection)
	}
}

func noteTodosHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		noteID, ok := parsePositivePathID(w, r, "note")
		if !ok {
			return
		}
		items, err := app.ListNoteTodosContext(r.Context(), noteID)
		if err != nil {
			writeOrganizerError(w, err, "note", "note_tasks_failed", "Linked tasks could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func todoNotesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		todoID, ok := parsePositivePathID(w, r, "task")
		if !ok {
			return
		}
		items, err := app.ListTodoNotesContext(r.Context(), todoID)
		if err != nil {
			writeOrganizerError(w, err, "task", "task_notes_failed", "Linked notes could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func linkNoteTodoHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		noteID, ok := parsePositivePathID(w, r, "note")
		if !ok {
			return
		}
		var request noteTodoWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		result, err := app.LinkNoteTodoWithStatusContext(r.Context(), backend.NoteTodoConnectionRequest{NoteID: noteID, TodoID: request.TodoID})
		if err != nil {
			writeOrganizerError(w, err, "note", "note_task_link_failed", "Task could not be linked.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func unlinkNoteTodoHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		noteID, ok := parsePositivePathID(w, r, "note")
		if !ok {
			return
		}
		todoID, err := strconv.Atoi(r.PathValue("todoID"))
		if err != nil || todoID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_task_id", "Task ID must be a positive integer.")
			return
		}
		result, err := app.UnlinkNoteTodoWithStatusContext(r.Context(), backend.NoteTodoConnectionRequest{NoteID: noteID, TodoID: todoID})
		if err != nil {
			writeOrganizerError(w, err, "note", "note_task_unlink_failed", "Task could not be unlinked.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func parseOptionalRevision(w http.ResponseWriter, r *http.Request) (*int, bool) {
	value := strings.TrimSpace(r.URL.Query().Get("expected_revision"))
	if value == "" {
		return nil, true
	}
	revision, err := strconv.Atoi(value)
	if err != nil || revision < 1 {
		writeError(w, http.StatusBadRequest, "invalid_expected_revision", "Expected revision must be a positive integer.")
		return nil, false
	}
	return &revision, true
}
