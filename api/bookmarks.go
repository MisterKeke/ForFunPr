package api

import (
	"net/http"
	"strings"

	"currency-wails/backend"
)

type bookmarkCreateWriteRequest struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type bookmarkUpdateWriteRequest struct {
	URL              string   `json:"url"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	ExpectedRevision int      `json:"expected_revision"`
}

type bookmarkReadWriteRequest struct {
	Read             bool `json:"read"`
	ExpectedRevision int  `json:"expected_revision"`
}

func bookmarksHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseNonNegativeQueryInteger(w, r, "limit")
		if !ok {
			return
		}
		offset, ok := parseNonNegativeQueryInteger(w, r, "offset")
		if !ok {
			return
		}
		result, err := app.ListBookmarksContext(r.Context(), backend.BookmarkFilter{
			Query: strings.TrimSpace(r.URL.Query().Get("q")),
			Status: strings.TrimSpace(r.URL.Query().Get("status")),
			Tags: r.URL.Query()["tag"],
			Limit: limit,
			Offset: offset,
		})
		if err != nil {
			writeOrganizerError(w, err, "bookmark", "bookmarks_failed", "Bookmarks could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func bookmarkHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "bookmark")
		if !ok {
			return
		}
		bookmark, err := app.GetBookmarkContext(r.Context(), id)
		if err != nil {
			writeOrganizerError(w, err, "bookmark", "bookmark_failed", "Bookmark could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, bookmark)
	}
}

func bookmarkTagsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags, err := app.ListBookmarkTagsContext(r.Context())
		if err != nil {
			writeOrganizerError(w, err, "bookmark", "bookmark_tags_failed", "Bookmark tags could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, tags)
	}
}

func createBookmarkHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request bookmarkCreateWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		bookmark, err := app.CreateBookmarkContext(r.Context(), backend.BookmarkCreateRequest{
			URL: request.URL, Title: request.Title,
			Description: request.Description, Tags: request.Tags,
		})
		if err != nil {
			writeOrganizerError(w, err, "bookmark", "bookmark_create_failed", "Bookmark could not be created.")
			return
		}
		app.EmitBookmarksChanged()
		writeJSON(w, http.StatusCreated, bookmark)
	}
}

func updateBookmarkHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "bookmark")
		if !ok {
			return
		}
		var request bookmarkUpdateWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		bookmark, err := app.UpdateBookmarkContext(r.Context(), backend.BookmarkUpdateRequest{
			ID: id, URL: request.URL, Title: request.Title,
			Description: request.Description, Tags: request.Tags,
			ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeOrganizerError(w, err, "bookmark", "bookmark_update_failed", "Bookmark could not be updated.")
			return
		}
		app.EmitBookmarksChanged()
		writeJSON(w, http.StatusOK, bookmark)
	}
}

func setBookmarkReadHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "bookmark")
		if !ok {
			return
		}
		var request bookmarkReadWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		bookmark, err := app.SetBookmarkReadContext(r.Context(), backend.BookmarkReadRequest{
			ID: id, Read: request.Read, ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeOrganizerError(w, err, "bookmark", "bookmark_read_failed", "Bookmark read state could not be changed.")
			return
		}
		app.EmitBookmarksChanged()
		writeJSON(w, http.StatusOK, bookmark)
	}
}

func deleteBookmarkHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePositivePathID(w, r, "bookmark")
		if !ok {
			return
		}
		if err := app.DeleteBookmarkContext(r.Context(), id); err != nil {
			writeOrganizerError(w, err, "bookmark", "bookmark_delete_failed", "Bookmark could not be deleted.")
			return
		}
		app.EmitBookmarksChanged()
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Bookmark deleted."})
	}
}
