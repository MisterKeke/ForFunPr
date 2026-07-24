package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) getJSON(
	ctx context.Context,
	path string,
	query url.Values,
	destination any,
) error {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	endpoint.RawPath = ""
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint.String(),
		nil,
	)
	if err != nil {
		return fmt.Errorf("create API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call %s: %w", endpoint.Redacted(), err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return decodeError(response)
	}

	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode API response: %w", err)
	}

	return nil
}
