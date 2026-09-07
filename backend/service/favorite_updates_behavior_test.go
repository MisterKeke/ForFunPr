package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFavoriteUpdateTimeWindowsAndCanonicalTelegramURLs(t *testing.T) {
	checked := "2026-01-02T10:00:00Z"
	scan := time.Date(2026, 1, 2, 11, 0, 0, 0, time.UTC)
	cases := []struct {
		published time.Time
		want      bool
	}{
		{time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC), false},
		{time.Date(2026, 1, 2, 10, 0, 1, 0, time.UTC), true},
		{scan, true},
		{scan.Add(time.Nanosecond), false},
	}
	for _, testCase := range cases {
		if got := favoriteUpdateInWindow(testCase.published, checked, scan); got != testCase.want {
			t.Fatalf("favoriteUpdateInWindow(%s) = %v, want %v", testCase.published, got, testCase.want)
		}
	}
	if got := maxFavoriteUpdateTimestamp("invalid", "2026-01-02T12:00:00+02:00"); got != checked {
		t.Fatalf("maximum timestamp = %q", got)
	}
	if !favoriteUpdateTimeBefore("2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z") {
		t.Fatal("timestamp ordering failed")
	}
	if got := TelegramPostURL("@Example_Channel", "example_channel/42"); got != "https://t.me/example_channel/42" {
		t.Fatalf("Telegram post URL = %q", got)
	}
	for _, invalid := range []string{"", "abc", "channel/1/2"} {
		if got := TelegramPostURL("valid_channel", invalid); got != "" {
			t.Fatalf("invalid post ID %q produced %q", invalid, got)
		}
	}
}

func TestFavoriteUpdatePersistenceDeduplicatesAndAdvancesOnlyOnSuccess(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	source := favoriteUpdateSource{
		Source: favoriteSourceTelegram, SourceID: "sample_channel",
		AddedAt: "2026-01-02T09:00:00Z",
	}
	scan := time.Date(2026, 1, 2, 11, 0, 0, 0, time.UTC)
	fetch := telegramFavoriteFetch{
		source: source, checkedThrough: "2026-01-02T10:00:00Z", sourceHasSeenItems: true,
		posts: []TelegramPost{{
			PostID: "sample_channel/42", Date: "2026-01-02T10:30:00Z", Text: "new post",
		}},
	}
	updates, err := service.persistTelegramFavoriteUpdateSource(ctx, fetch, fetch.checkedThrough, scan)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].PostURL != "https://t.me/sample_channel/42" {
		t.Fatalf("persisted updates = %#v", updates)
	}

	var checkpoint string
	if err := service.db.QueryRow(`SELECT checked_through FROM favorite_update_checkpoints
		WHERE source = ? AND source_id = ?`, source.Source, source.SourceID).Scan(&checkpoint); err != nil {
		t.Fatal(err)
	}
	if checkpoint != scan.Format(time.RFC3339) {
		t.Fatalf("checkpoint = %q", checkpoint)
	}

	fetch.checkedThrough = checkpoint
	fetch.sourceHasSeenItems = true
	secondScan := scan.Add(time.Hour)
	updates, err = service.persistTelegramFavoriteUpdateSource(ctx, fetch, fetch.checkedThrough, secondScan)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 0 {
		t.Fatalf("replayed item was considered new: %#v", updates)
	}
	if err := service.db.QueryRow(`SELECT checked_through FROM favorite_update_checkpoints
		WHERE source = ? AND source_id = ?`, source.Source, source.SourceID).Scan(&checkpoint); err != nil {
		t.Fatal(err)
	}
	if checkpoint != secondScan.Format(time.RFC3339) {
		t.Fatalf("second checkpoint = %q", checkpoint)
	}

	failureTime := scan.Add(2 * time.Hour)
	if err := recordFavoriteUpdateFailure(service.db, source, checkpoint, failureTime, errors.New("provider secret")); err != nil {
		t.Fatal(err)
	}
	var afterFailure, lastError string
	if err := service.db.QueryRow(`SELECT checked_through, last_error FROM favorite_update_checkpoints
		WHERE source = ? AND source_id = ?`, source.Source, source.SourceID).Scan(&afterFailure, &lastError); err != nil {
		t.Fatal(err)
	}
	if afterFailure != checkpoint || lastError != "provider secret" {
		t.Fatalf("failure checkpoint/error = (%q, %q)", afterFailure, lastError)
	}
}

func TestFavoriteUpdateScanUsesProviderResultsAndDoesNotReplayItems(t *testing.T) {
	service := newFeatureTestService(t)
	now := time.Now().UTC()
	published := now.Add(-time.Hour).Format(time.RFC3339)
	old := now.Add(-24 * time.Hour).Format(time.RFC3339)
	if _, err := service.db.Exec(`INSERT INTO telegram_favorites(username, added_at) VALUES (?, ?)`, "sample_channel", old); err != nil {
		t.Fatal(err)
	}
	channelID := "UC0000000000000000000000"
	if _, err := service.db.Exec(`INSERT INTO youtube_favorites(channel_id, added_at) VALUES (?, ?)`, channelID, old); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"previous_opened_at":  old,
		"current_opened_at":   old,
		"previous_refresh_at": old,
		"last_refresh_at":     old,
	} {
		if err := upsertAppStateValue(context.Background(), service.db, key, value); err != nil {
			t.Fatal(err)
		}
	}

	service.httpClient.client.Transport = testRoundTripper(func(request *http.Request) (*http.Response, error) {
		var body string
		switch request.URL.Hostname() {
		case "t.me":
			body = fmt.Sprintf(`<div class="tgme_widget_message_wrap" data-post="sample_channel/42">
				<div class="tgme_widget_message_text">Telegram update</div>
				<a class="tgme_widget_message_date"><time datetime="%s"></time></a></div>`, published)
		case "www.youtube.com":
			body = fmt.Sprintf(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"
				xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns:media="http://search.yahoo.com/mrss/">
				<entry><yt:videoId>abcdefghijk</yt:videoId><title>YouTube update</title><published>%s</published>
				<author><name>Example</name></author><media:group><media:thumbnail url="https://example.com/thumb.jpg"/>
				<media:description>Description</media:description></media:group></entry></feed>`, published)
		default:
			return nil, fmt.Errorf("unexpected provider %s", request.URL.Hostname())
		}
		return &http.Response{
			StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(body)), Request: request,
		}, nil
	})

	first, err := service.GetInitialFavoriteUpdatesContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.NewUpdates) != 2 || len(first.Errors) != 0 {
		t.Fatalf("first scan = %#v", first)
	}
	second, err := service.GetInitialFavoriteUpdatesContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(second.NewUpdates) != 0 {
		t.Fatalf("second scan replayed updates: %#v", second.NewUpdates)
	}
}

func TestFavoriteProviderErrorsAreSafeForUsers(t *testing.T) {
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	message := safeFavoriteProviderError("Provider", errors.New("token=super-secret"))
	if strings.Contains(message, "super-secret") || message != "Provider data could not be loaded." {
		t.Fatalf("safe provider error = %q", message)
	}
	if got := safeFavoriteProviderError("Provider", context.DeadlineExceeded); got != "Provider request timed out." {
		t.Fatalf("timeout message = %q", got)
	}
}
