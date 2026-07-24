package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encoding_error", "The response could not be encoded.")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(append(payload, '\n'))
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	payload, err := json.Marshal(errorBody{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(append(payload, '\n'))
}

func writeNoContent(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

const maximumJSONBodyBytes int64 = 1 << 20 // 1 MiB

func decodeJSONBody(
	w http.ResponseWriter,
	r *http.Request,
	destination any,
) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil ||
		(mediaType != "application/json" &&
			!strings.HasSuffix(mediaType, "+json")) {
		writeError(
			w,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content-Type must be application/json.",
		)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maximumJSONBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		var tooLarge *http.MaxBytesError

		switch {
		case errors.As(err, &tooLarge):
			writeError(
				w,
				http.StatusRequestEntityTooLarge,
				"request_too_large",
				"Request body is too large.",
			)

		case errors.Is(err, io.EOF):
			writeError(
				w,
				http.StatusBadRequest,
				"missing_json_body",
				"Request body must contain JSON.",
			)

		default:
			writeError(
				w,
				http.StatusBadRequest,
				"invalid_json",
				"Request body contains invalid JSON.",
			)
		}

		return false
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError

		if errors.As(err, &tooLarge) {
			writeError(
				w,
				http.StatusRequestEntityTooLarge,
				"request_too_large",
				"Request body is too large.",
			)
		} else {
			writeError(
				w,
				http.StatusBadRequest,
				"invalid_json",
				"Request body must contain exactly one JSON object.",
			)
		}

		return false
	}

	return true
}
