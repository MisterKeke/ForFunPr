package schemas

// HealthOutput is the JSON emitted by `something health`.
type HealthOutput struct {
	Ready bool   `json:"ready" jsonschema:"Whether the desktop backend and local storage are ready."`
	Error string `json:"error,omitempty" jsonschema:"Safe startup error message when the backend is not ready."`
}
