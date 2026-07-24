package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) doJSON(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
	destination any,
) error {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	endpoint.RawPath = ""
	endpoint.RawQuery = query.Encode()

	var body io.Reader
	if requestBody != nil {
		var encoded bytes.Buffer
		if err := json.NewEncoder(&encoded).Encode(requestBody); err != nil {
			return fmt.Errorf("encode API request: %w", err)
		}
		body = &encoded
	}

	request, err := http.NewRequestWithContext(
		ctx,
		method,
		endpoint.String(),
		body,
	)
	if err != nil {
		return fmt.Errorf("create API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call %s: %w", endpoint.Redacted(), err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return decodeError(response)
	}

	if destination == nil {
		return nil
	}

	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode API response: %w", err)
	}

	return nil
}

func (c *Client) doValue(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
) (any, error) {
	var result any
	if err := c.doJSON(
		ctx,
		method,
		path,
		query,
		requestBody,
		&result,
	); err != nil {
		return nil, err
	}

	return result, nil
}
