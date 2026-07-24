package apiclient

import (
	"context"
	"net/http"
)

func (c *Client) Health(ctx context.Context) (any, error) {
	return c.doValue(ctx, http.MethodGet, "/api/v1/health", nil, nil)
}
