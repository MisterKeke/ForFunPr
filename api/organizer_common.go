package api

import (
	"net/http"
	"strconv"
	"strings"
)

func parsePositivePathID(w http.ResponseWriter, r *http.Request, resource string) (int, bool) {
	value := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_"+resource+"_id",
			strings.ToUpper(resource[:1])+resource[1:]+" ID must be a positive integer.",
		)
		return 0, false
	}
	return id, true
}

func parseNonNegativeQueryInteger(
	w http.ResponseWriter,
	r *http.Request,
	name string,
) (int, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		writeError(w, http.StatusBadRequest, "invalid_"+name, name+" must be zero or greater.")
		return 0, false
	}
	return parsed, true
}
