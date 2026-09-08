package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	backend "something/backend/service"
)

type noteCreateWriteRequest struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Pinned bool   `json:"pinned,omitempty"`
}

type noteUpdateWriteRequest struct {
	Title            string `json:"title"`
	Body             string `json:"body"`
	ExpectedRevision int    `json:"expected_revision"`
}

type noteStateWriteRequest struct {
	Value            bool `json:"value"`
	ExpectedRevision int  `json:"expected_revision"`
}

func notesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseNonNegativeQueryInteger(w, r, "limit")
		if !ok {
			return
		}
		offset, ok := parseNonNegativeQueryInteger(w, r, "offset")
		if !ok {
			return
		}
		filter := backend.NoteListFilter{
			Query:         strings.TrimSpace(r.URL.Query().Get("q")),
			ArchiveStatus: strings.TrimSpace(r.URL.Query().Get("archive")),
			Limit:         limit,
			Offset:        offset,
		}
		if value := strings.TrimSpace(r.URL.Query().Get("pinned")); value != "" {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_pinned", "Pinned must be true or false.")
				return
			}
			filter.Pinned = &parsed
		}
		result, err := app.ListNotesContext(r.Context(), filter)
		if err != nil {
			writeOrganizerError(w, err, "note", "notes_failed", "Notes could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func noteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note")
		if !ok {
			return
		}
		note, err := app.GetNoteContext(r.Context(), id)
		if err != nil {
			writeOrganizerError(w, err, "note", "note_failed", "Note could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, note)
	}
}

func createNoteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request noteCreateWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		note, err := app.CreateNoteContext(r.Context(), backend.NoteCreateRequest{
			Title: request.Title, Body: request.Body, Pinned: request.Pinned,
		})
		if err != nil {
			writeOrganizerError(w, err, "note", "note_create_failed", "Note could not be created.")
			return
		}
		app.EmitNotesChanged()
		writeJSON(w, http.StatusCreated, note)
	}
}

func updateNoteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note")
		if !ok {
			return
		}
		var request noteUpdateWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		note, err := app.UpdateNoteContext(r.Context(), backend.NoteUpdateRequest{
			ID: id, Title: request.Title, Body: request.Body,
			ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeOrganizerError(w, err, "note", "note_update_failed", "Note could not be updated.")
			return
		}
		app.EmitNotesChanged()
		writeJSON(w, http.StatusOK, note)
	}
}

func setNotePinnedHandler(app *backend.Service) http.HandlerFunc {
	return setNoteStateHandler(app, true)
}

func setNoteArchivedHandler(app *backend.Service) http.HandlerFunc {
	return setNoteStateHandler(app, false)
}

func setNoteStateHandler(app *backend.Service, pinned bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note")
		if !ok {
			return
		}
		var request noteStateWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		stateRequest := backend.NoteStateRequest{
			ID: id, Value: request.Value, ExpectedRevision: request.ExpectedRevision,
		}
		var note backend.Note
		var err error
		if pinned {
			note, err = app.SetNotePinnedContext(r.Context(), stateRequest)
		} else {
			note, err = app.SetNoteArchivedContext(r.Context(), stateRequest)
		}
		if err != nil {
			writeOrganizerError(w, err, "note", "note_state_failed", "Note state could not be changed.")
			return
		}
		app.EmitNotesChanged()
		writeJSON(w, http.StatusOK, note)
	}
}

func deleteNoteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "note")
		if !ok {
			return
		}
		if err := app.DeleteNoteContext(r.Context(), id); err != nil {
			writeOrganizerError(w, err, "note", "note_delete_failed", "Note could not be deleted.")
			return
		}
		app.EmitNotesChanged()
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Note deleted."})
	}
}

func writeOrganizerError(
	w http.ResponseWriter,
	err error,
	resource string,
	fallbackCode string,
	fallbackMessage string,
) {
	var validation *backend.ValidationError
	if errors.As(err, &validation) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_"+resource, validation.Message)
		return
	}
	var notFound *backend.NotFoundError
	if errors.As(err, &notFound) {
		writeError(w, http.StatusNotFound, resource+"_not_found", "The requested "+resource+" does not exist.")
		return
	}
	var conflict *backend.ConflictError
	if errors.As(err, &conflict) {
		writeError(w, http.StatusConflict, resource+"_conflict", conflict.Message)
		return
	}
	var stale *backend.StaleRevisionError
	if errors.As(err, &stale) {
		writeError(w, http.StatusConflict, resource+"_stale_revision", "The resource changed; reload it and retry.")
		return
	}
	writeError(w, http.StatusInternalServerError, fallbackCode, fallbackMessage)
}
