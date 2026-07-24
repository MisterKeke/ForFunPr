package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"currency-wails/cli/internal/apiclient"
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

type channelDateOutput struct {
	Date        string `json:"date"`
	ChannelName string `json:"channel_name"`
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

// News intentionally exposes only date and channel name in both formats.
func writeNewsOutput(
	output io.Writer,
	format string,
	response apiclient.NewsResponse,
) error {
	items := make([]channelDateOutput, 0, len(response.News))
	rows := make([][]string, 0, len(response.News))

	for _, item := range response.News {
		projected := channelDateOutput{
			Date:        item.PostedAt,
			ChannelName: item.ChannelName,
		}
		items = append(items, projected)
		rows = append(rows, []string{
			projected.Date,
			projected.ChannelName,
		})
	}

	jsonValue := struct {
		News []channelDateOutput `json:"news"`
	}{News: items}

	return writeFormattedOutput(output, format, jsonValue, tableData{
		headers:      []string{"DATE", "CHANNEL"},
		rows:         rows,
		emptyMessage: "No news found.",
	})
}

// Post tables show all available API and command context. Post JSON output is
// intentionally limited to date and channel name.
func writePostsOutput(
	output io.Writer,
	format string,
	source string,
	channel string,
	posts []apiclient.Post,
) error {
	jsonItems := make([]channelDateOutput, 0, len(posts))
	rows := make([][]string, 0, len(posts))

	for _, post := range posts {
		postSource := post.Source
		if postSource == "" {
			postSource = source
		}

		postChannel := post.ChannelName
		if postChannel == "" {
			postChannel = channel
		}

		jsonItems = append(jsonItems, channelDateOutput{
			Date:        post.PostedAt,
			ChannelName: postChannel,
		})

		switch source {
		case "telegram":
			rows = append(rows, []string{
				postSource,
				post.PostedAt,
				postChannel,
				post.Text,
				strings.Join(post.Images, ", "),
				post.Views,
				post.PostID,
			})
		case "youtube":
			rows = append(rows, []string{
				postSource,
				post.PostedAt,
				postChannel,
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
	}

	headers := []string{
		"SOURCE",
		"DATE",
		"CHANNEL",
		"TEXT",
		"IMAGES",
		"VIEWS",
		"POST_ID",
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
