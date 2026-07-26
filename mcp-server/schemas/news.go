package schemas

// NewsError records one favorite source that could not be refreshed.
type NewsError struct {
	Source   string `json:"source" jsonschema:"Provider source: telegram or youtube."`
	SourceID string `json:"source_id,omitempty" jsonschema:"Favorite channel identifier when available."`
	Error    string `json:"error" jsonschema:"Source-specific refresh error."`
}

// NewsOutput is the JSON emitted by news list and scan commands.
type NewsOutput struct {
	ScanStartedAt string          `json:"scan_started_at" jsonschema:"UTC timestamp at which the scan started."`
	News          []Post          `json:"news" jsonschema:"Full Telegram and YouTube news posts, newest first."`
	Errors        []NewsError     `json:"errors" jsonschema:"Favorite sources that failed during the scan."`
	State         NewsStateOutput `json:"state" jsonschema:"Application-open and refresh state after the scan."`
}

// UpdateWindow is one publication interval in the news state.
type UpdateWindow struct {
	PublishedAfter string `json:"published_after" jsonschema:"Exclusive lower publication timestamp."`
	PublishedUntil string `json:"published_until" jsonschema:"Inclusive upper publication timestamp."`
}

// UpdateWindowsOutput is the JSON emitted by `something news windows`.
type UpdateWindowsOutput struct {
	NewWhileClosed UpdateWindow `json:"new_while_closed" jsonschema:"Publication window while the app was closed."`
	NewWhileOpen   UpdateWindow `json:"new_while_open" jsonschema:"Publication window since the previous refresh."`
}

// NewsStateOutput is the JSON emitted by `something news state`.
type NewsStateOutput struct {
	PreviousOpenedAt  string              `json:"previous_opened_at" jsonschema:"Previous application-open timestamp."`
	CurrentOpenedAt   string              `json:"current_opened_at" jsonschema:"Current application-open timestamp."`
	PreviousRefreshAt string              `json:"previous_refresh_at" jsonschema:"Previous news refresh timestamp."`
	LastRefreshAt     string              `json:"last_refresh_at" jsonschema:"Latest news refresh timestamp."`
	UpdateWindows     UpdateWindowsOutput `json:"update_windows" jsonschema:"Derived news publication windows."`
}
