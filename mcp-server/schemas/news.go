package schemas

// ChannelDate is the intentionally reduced JSON item emitted by news and post
// CLI commands.
type ChannelDate struct {
	Date        string `json:"date" jsonschema:"Publication timestamp returned by the provider."`
	ChannelName string `json:"channel_name" jsonschema:"Telegram or YouTube channel name."`
}

// NewsOutput is the JSON emitted by news list and scan commands.
type NewsOutput struct {
	News []ChannelDate `json:"news" jsonschema:"Projected news items, newest first."`
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
