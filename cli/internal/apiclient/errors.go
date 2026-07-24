package apiclient

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Error represents the standard error envelope returned by the HTTP API.
type Error struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *Error) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("API returned HTTP %d: %s", e.StatusCode, e.Message)
	}

	return fmt.Sprintf("API returned HTTP %d (%s): %s", e.StatusCode, e.Code, e.Message)
}

func decodeError(response *http.Response) error {
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return &Error{
			StatusCode: response.StatusCode,
			Message:    http.StatusText(response.StatusCode),
		}
	}

	return &Error{
		StatusCode: response.StatusCode,
		Code:       body.Error.Code,
		Message:    body.Error.Message,
	}
}
