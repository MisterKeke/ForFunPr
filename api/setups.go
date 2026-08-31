package api

import (
	"errors"
	"net/http"

	backend "something/backend/service"
)

type setupCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AppIDs      []int  `json:"app_ids"`
}

type setupUpdateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AppIDs      []int  `json:"app_ids"`
	RemoveIcon  bool   `json:"remove_icon,omitempty"`
}

func setupsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		items, err := app.ListSetupsContext(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "setups_failed", "Setups could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func createSetupHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		var request setupCreateRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		item, err := app.CreateSetupContext(r.Context(), backend.SetupCreateRequest{
			Name: request.Name, Description: request.Description, AppIDs: request.AppIDs,
		})
		if err != nil {
			writeSetupError(w, err, "setups_create_failed", "The setup could not be created.")
			return
		}
		app.EmitSetupsChanged()
		writeJSON(w, http.StatusCreated, item)
	}
}

func updateSetupHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		id, ok := parsePositivePathID(w, r, "setup")
		if !ok {
			return
		}
		var request setupUpdateRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		item, err := app.UpdateSetupContext(r.Context(), backend.SetupUpdateRequest{
			ID: id, Name: request.Name, Description: request.Description,
			AppIDs: request.AppIDs, RemoveIcon: request.RemoveIcon,
		})
		if err != nil {
			writeSetupError(w, err, "setups_update_failed", "The setup could not be updated.")
			return
		}
		app.EmitSetupsChanged()
		writeJSON(w, http.StatusOK, item)
	}
}

func deleteSetupHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		id, ok := parsePositivePathID(w, r, "setup")
		if !ok {
			return
		}
		if err := app.DeleteSetupContext(r.Context(), id); err != nil {
			writeSetupError(w, err, "setups_delete_failed", "The setup could not be deleted.")
			return
		}
		app.EmitSetupsChanged()
		writeNoContent(w)
	}
}

func writeSetupError(w http.ResponseWriter, err error, fallbackCode string, fallbackMessage string) {
	var validation *backend.ValidationError
	var notFound *backend.NotFoundError
	var conflict *backend.ConflictError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusUnprocessableEntity, "invalid_setup", validation.Message)
	case errors.As(err, &notFound):
		writeError(w, http.StatusNotFound, "setup_not_found", "The requested setup or one of its applications does not exist.")
	case errors.As(err, &conflict):
		writeError(w, http.StatusConflict, "setup_conflict", conflict.Message)
	default:
		writeError(w, http.StatusInternalServerError, fallbackCode, fallbackMessage)
	}
}
