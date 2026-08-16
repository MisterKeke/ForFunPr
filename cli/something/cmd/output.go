package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"something/cli/internal/apiclient"
)

const (
	outputConfigKey     = "output"
	defaultOutputFormat = "table"
)

type tableData struct {
	headers      []string
	rows         [][]string
	emptyMessage string
}

type postOutput struct {
	Source       string   `json:"source"`
	Date         string   `json:"date"`
	ChannelName  string   `json:"channel_name"`
	Views        string   `json:"views,omitempty"`
	Text         string   `json:"text,omitempty"`
	Images       []string `json:"images,omitempty"`
	PostID       string   `json:"post_id,omitempty"`
	PostURL      string   `json:"post_url,omitempty"`
	VideoID      string   `json:"video_id,omitempty"`
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	Thumbnail    string   `json:"thumbnail,omitempty"`
	ChannelID    string   `json:"channel_id,omitempty"`
	ChannelTitle string   `json:"channel_title,omitempty"`
	VideoURL     string   `json:"video_url,omitempty"`
	Duration     string   `json:"duration,omitempty"`
}

type newsOutput struct {
	ScanStartedAt string                `json:"scan_started_at"`
	News          []postOutput          `json:"news"`
	NewNews       []postOutput          `json:"new_news"`
	Errors        []apiclient.NewsError `json:"errors"`
	State         apiclient.NewsState   `json:"state"`
}

func writeValueOutput(
	output io.Writer,
	format string,
	value any,
) error {
	table, err := tableFromValue(value)
	if err != nil {
		return err
	}

	return writeFormattedOutput(output, format, value, table)
}

func writeSuccessOutput(
	output io.Writer,
	format string,
	message string,
) error {
	return writeValueOutput(output, format, map[string]string{
		"status":  "ok",
		"message": message,
	})
}

// News JSON preserves full source-specific post fields and scan metadata. The
// table remains compact for interactive terminal use.
func writeNewsOutput(
	output io.Writer,
	format string,
	response apiclient.NewsResponse,
) error {
	items := make([]postOutput, 0, len(response.News))
	newItems := make([]postOutput, 0, len(response.NewNews))
	rows := make([][]string, 0, len(response.News))

	for _, item := range response.News {
		outputItem := postOutputFromAPI(item, item.Source, item.ChannelName)
		items = append(items, outputItem)
		rows = append(rows, []string{
			outputItem.Source,
			outputItem.Date,
			outputItem.ChannelName,
		})
	}
	for _, item := range response.NewNews {
		newItems = append(newItems, postOutputFromAPI(item, item.Source, item.ChannelName))
	}

	return writeFormattedOutput(output, format, newsOutput{
		ScanStartedAt: response.ScanStartedAt,
		News:          items,
		NewNews:       newItems,
		Errors:        response.Errors,
		State:         response.State,
	}, tableData{
		headers:      []string{"SOURCE", "DATE", "CHANNEL"},
		rows:         rows,
		emptyMessage: "No news found.",
	})
}

// Post output includes common fields and the fields specific to its source.
func writePostsOutput(
	output io.Writer,
	format string,
	source string,
	channel string,
	posts []apiclient.Post,
) error {
	jsonItems := make([]postOutput, 0, len(posts))
	rows := make([][]string, 0, len(posts))

	for _, post := range posts {
		jsonItem := postOutputFromAPI(post, source, channel)

		switch jsonItem.Source {
		case "telegram":
			rows = append(rows, []string{
				jsonItem.Source,
				post.PostedAt,
				jsonItem.ChannelName,
				post.Text,
				strings.Join(post.Images, ", "),
				post.Views,
				post.PostID,
				post.PostURL,
			})
		case "youtube":
			rows = append(rows, []string{
				jsonItem.Source,
				post.PostedAt,
				jsonItem.ChannelName,
				post.VideoID,
				post.Title,
				post.Description,
				post.Thumbnail,
				post.ChannelID,
				post.ChannelTitle,
				post.VideoURL,
				post.Views,
				post.Duration,
			})
		}

		jsonItems = append(jsonItems, jsonItem)
	}

	headers := []string{
		"SOURCE",
		"DATE",
		"CHANNEL",
		"TEXT",
		"IMAGES",
		"VIEWS",
		"POST_ID",
		"POST_URL",
	}
	if source == "youtube" {
		headers = []string{
			"SOURCE",
			"DATE",
			"CHANNEL",
			"VIDEO_ID",
			"TITLE",
			"DESCRIPTION",
			"THUMBNAIL",
			"CHANNEL_ID",
			"CHANNEL_TITLE",
			"VIDEO_URL",
			"VIEWS",
			"DURATION",
		}
	}

	return writeFormattedOutput(output, format, jsonItems, tableData{
		headers:      headers,
		rows:         rows,
		emptyMessage: "No posts found.",
	})
}

func postOutputFromAPI(
	post apiclient.Post,
	fallbackSource string,
	fallbackChannel string,
) postOutput {
	postSource := strings.ToLower(strings.TrimSpace(post.Source))
	if postSource == "" {
		postSource = strings.ToLower(strings.TrimSpace(fallbackSource))
	}

	postChannel := strings.TrimSpace(post.ChannelName)
	if postChannel == "" {
		postChannel = strings.TrimSpace(fallbackChannel)
	}

	item := postOutput{
		Source:      postSource,
		Date:        post.PostedAt,
		ChannelName: postChannel,
		Views:       post.Views,
	}

	switch postSource {
	case "telegram":
		item.Text = post.Text
		item.Images = post.Images
		item.PostID = post.PostID
		item.PostURL = post.PostURL
	case "youtube":
		item.VideoID = post.VideoID
		item.Title = post.Title
		item.Description = post.Description
		item.Thumbnail = post.Thumbnail
		item.ChannelID = post.ChannelID
		item.ChannelTitle = post.ChannelTitle
		item.VideoURL = post.VideoURL
		item.Duration = post.Duration
	}

	return item
}

func writeFormattedOutput(
	output io.Writer,
	format string,
	jsonValue any,
	table tableData,
) error {
	if err := validateOutputFormat(format); err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return writeJSONOutput(output, jsonValue)
	case "table":
		return writeTable(output, table)
	}

	return nil
}

func validateOutputFormat(format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "table":
		return nil
	default:
		return fmt.Errorf("output format must be table or json")
	}
}

func writeJSONOutput(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeTable(output io.Writer, data tableData) error {
	if len(data.rows) == 0 {
		message := data.emptyMessage
		if message == "" {
			message = "No results."
		}
		_, err := fmt.Fprintln(output, message)
		return err
	}

	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(
		table,
		strings.Join(data.headers, "\t"),
	); err != nil {
		return err
	}

	for _, row := range data.rows {
		if len(row) != len(data.headers) {
			return fmt.Errorf(
				"table row has %d cells; expected %d",
				len(row),
				len(data.headers),
			)
		}

		cells := make([]string, len(row))
		for index, value := range row {
			cells[index] = normalizeTableCell(value)
		}

		if _, err := fmt.Fprintln(
			table,
			strings.Join(cells, "\t"),
		); err != nil {
			return err
		}
	}

	return table.Flush()
}

func tableFromValue(value any) (tableData, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return tableData{}, fmt.Errorf("encode table value: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()

	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return tableData{}, fmt.Errorf("decode table value: %w", err)
	}

	return tableFromNormalizedValue(normalized)
}

func tableFromNormalizedValue(value any) (tableData, error) {
	switch typed := value.(type) {
	case []any:
		return tableFromSlice(typed), nil
	case map[string]any:
		if len(typed) == 1 {
			for _, nested := range typed {
				if values, ok := nested.([]any); ok {
					return tableFromSlice(values), nil
				}
			}
		}

		keys := sortedMapKeys(typed)
		row := make([]string, 0, len(keys))
		headers := make([]string, 0, len(keys))
		for _, key := range keys {
			headers = append(headers, strings.ToUpper(key))
			row = append(row, formatTableValue(typed[key]))
		}

		return tableData{
			headers:      headers,
			rows:         [][]string{row},
			emptyMessage: "No results.",
		}, nil
	case nil:
		return tableData{
			headers:      []string{"VALUE"},
			emptyMessage: "No results.",
		}, nil
	default:
		return tableData{
			headers:      []string{"VALUE"},
			rows:         [][]string{{formatTableValue(typed)}},
			emptyMessage: "No results.",
		}, nil
	}
}

func tableFromSlice(values []any) tableData {
	if len(values) == 0 {
		return tableData{
			headers:      []string{"VALUE"},
			emptyMessage: "No results.",
		}
	}

	keySet := make(map[string]struct{})
	allObjects := true
	for _, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			allObjects = false
			break
		}
		for key := range object {
			keySet[key] = struct{}{}
		}
	}

	if !allObjects {
		rows := make([][]string, 0, len(values))
		for _, value := range values {
			rows = append(rows, []string{formatTableValue(value)})
		}
		return tableData{
			headers:      []string{"VALUE"},
			rows:         rows,
			emptyMessage: "No results.",
		}
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	headers := make([]string, 0, len(keys))
	for _, key := range keys {
		headers = append(headers, strings.ToUpper(key))
	}

	rows := make([][]string, 0, len(values))
	for _, value := range values {
		object := value.(map[string]any)
		row := make([]string, 0, len(keys))
		for _, key := range keys {
			row = append(row, formatTableValue(object[key]))
		}
		rows = append(rows, row)
	}

	return tableData{
		headers:      headers,
		rows:         rows,
		emptyMessage: "No results.",
	}
}

func sortedMapKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatTableValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "-"
	case string:
		if typed == "" {
			return "-"
		}
		return typed
	case bool, json.Number:
		return fmt.Sprint(typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func normalizeTableCell(value string) string {
	return strings.NewReplacer(
		"\t", " ",
		"\r", " ",
		"\n", " ",
	).Replace(value)
}
