package cmd

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"something/cli/internal/apiclient"
)

func TestValidateOutputFormat(t *testing.T) {
	for _, value := range []string{"json", "JSON", " table "} {
		if err := validateOutputFormat(value); err != nil {
			t.Errorf("validateOutputFormat(%q) returned %v", value, err)
		}
	}
	for _, value := range []string{"", "yaml", "jsonl"} {
		if err := validateOutputFormat(value); err == nil {
			t.Errorf("validateOutputFormat(%q) returned no error", value)
		}
	}
}

func TestWriteJSONOutputIsIndentedWithoutHTMLEscaping(t *testing.T) {
	var output bytes.Buffer
	if err := writeJSONOutput(&output, map[string]string{"markup": "<b>&</b>"}); err != nil {
		t.Fatalf("writeJSONOutput returned %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "\n  \"markup\": \"<b>&</b>\"\n") {
		t.Fatalf("unexpected JSON output:\n%s", got)
	}
	if strings.Contains(got, "\\u003c") || strings.Contains(got, "\\u0026") {
		t.Fatalf("HTML characters were escaped:\n%s", got)
	}
}

func TestTableFromValueNormalizesShapes(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		headers []string
		rows    [][]string
	}{
		{
			name:    "object keys sorted",
			value:   map[string]any{"zeta": true, "alpha": 2},
			headers: []string{"ALPHA", "ZETA"},
			rows:    [][]string{{"2", "true"}},
		},
		{
			name: "object slice union",
			value: []map[string]any{
				{"name": "first", "id": 1},
				{"name": "second", "active": true},
			},
			headers: []string{"ACTIVE", "ID", "NAME"},
			rows: [][]string{
				{"-", "1", "first"},
				{"true", "-", "second"},
			},
		},
		{
			name:    "single nested slice",
			value:   map[string]any{"items": []map[string]any{{"id": 7}}},
			headers: []string{"ID"},
			rows:    [][]string{{"7"}},
		},
		{
			name:    "mixed slice",
			value:   []any{"value", 3, nil},
			headers: []string{"VALUE"},
			rows:    [][]string{{"value"}, {"3"}, {"-"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table, err := tableFromValue(test.value)
			if err != nil {
				t.Fatalf("tableFromValue returned %v", err)
			}
			if !reflect.DeepEqual(table.headers, test.headers) {
				t.Errorf("headers = %#v, want %#v", table.headers, test.headers)
			}
			if !reflect.DeepEqual(table.rows, test.rows) {
				t.Errorf("rows = %#v, want %#v", table.rows, test.rows)
			}
		})
	}
}

func TestTableFromValueRejectsUnsupportedJSONValue(t *testing.T) {
	if _, err := tableFromValue(make(chan int)); err == nil {
		t.Fatal("tableFromValue accepted a channel")
	}
}

func TestWriteTableHandlesEmptyRowsAndInvalidRows(t *testing.T) {
	var output bytes.Buffer
	if err := writeTable(&output, tableData{emptyMessage: "Nothing here."}); err != nil {
		t.Fatalf("writeTable empty returned %v", err)
	}
	if got := output.String(); got != "Nothing here.\n" {
		t.Fatalf("empty output = %q", got)
	}

	output.Reset()
	err := writeTable(&output, tableData{
		headers: []string{"ONE", "TWO"},
		rows:    [][]string{{"only one"}},
	})
	if err == nil || !strings.Contains(err.Error(), "expected 2") {
		t.Fatalf("row width error = %v", err)
	}
}

func TestWriteTableNormalizesControlCharacters(t *testing.T) {
	var output bytes.Buffer
	if err := writeTable(&output, tableData{
		headers: []string{"VALUE"},
		rows:    [][]string{{"one\ttwo\r\nthree"}},
	}); err != nil {
		t.Fatalf("writeTable returned %v", err)
	}

	got := output.String()
	if strings.Contains(got, "one\ttwo") || strings.Contains(got, "\r") {
		t.Fatalf("control characters were not normalized: %q", got)
	}
	if !strings.Contains(got, "one two  three") {
		t.Fatalf("normalized value missing from %q", got)
	}
}

func TestPostOutputFromAPIUsesFallbacksAndSourceFields(t *testing.T) {
	telegram := postOutputFromAPI(apiclient.Post{
		PostedAt: "2026-01-02T03:04:05Z",
		Text:     "message",
		Images:   []string{"one.jpg"},
		Views:    "12",
		PostID:   "42",
		PostURL:  "https://t.me/example/42",
	}, " TELEGRAM ", " example ")
	if telegram.Source != "telegram" || telegram.ChannelName != "example" ||
		telegram.Text != "message" || telegram.PostID != "42" || telegram.VideoID != "" {
		t.Fatalf("unexpected Telegram output: %#v", telegram)
	}

	youtube := postOutputFromAPI(apiclient.Post{
		Source:       "YouTube",
		ChannelName:  "Channel",
		VideoID:      "video",
		Title:        "title",
		Description:  "description",
		ChannelID:    "channel-id",
		ChannelTitle: "Channel title",
		VideoURL:     "https://youtube.example/watch?v=video",
		Duration:     "3:21",
		Text:         "must not leak",
	}, "telegram", "fallback")
	if youtube.Source != "youtube" || youtube.VideoID != "video" ||
		youtube.Duration != "3:21" || youtube.Text != "" || youtube.PostID != "" {
		t.Fatalf("unexpected YouTube output: %#v", youtube)
	}
}

func TestWritePostsOutputPreservesJSONFields(t *testing.T) {
	posts := []apiclient.Post{{
		PostedAt: "2026-01-02T03:04:05Z",
		Text:     "message",
		Images:   []string{"one.jpg", "two.jpg"},
		Views:    "12",
		PostID:   "42",
		PostURL:  "https://t.me/example/42",
	}}

	var output bytes.Buffer
	if err := writePostsOutput(&output, "json", "telegram", "example", posts); err != nil {
		t.Fatalf("writePostsOutput returned %v", err)
	}

	var decoded []postOutput
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Source != "telegram" ||
		decoded[0].ChannelName != "example" || decoded[0].PostURL != posts[0].PostURL ||
		!reflect.DeepEqual(decoded[0].Images, posts[0].Images) {
		t.Fatalf("unexpected posts output: %#v", decoded)
	}
}
