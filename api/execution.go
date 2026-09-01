package api

import "net/http"

type executionConfirmationRequest struct {
	Confirm bool `json:"confirm"`
}

func decodeExecutionConfirmation(w http.ResponseWriter, r *http.Request) bool {
	var request executionConfirmationRequest
	if !decodeJSONBody(w, r, &request) {
		return false
	}
	if !request.Confirm {
		writeError(
			w,
			http.StatusUnprocessableEntity,
			"execution_confirmation_required",
			"Set confirm to true to allow the application launch.",
		)
		return false
	}
	return true
}
