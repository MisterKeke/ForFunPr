package apiclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func New(rawBaseURL string) (*Client, error) {
	baseURL, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return nil, fmt.Errorf("parse API URL: %w", err)
	}
	if (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return nil, fmt.Errorf("API URL must be an absolute http or https URL")
	}
	if baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, fmt.Errorf("API URL must not contain credentials, a query, or a fragment")
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}
