package youtrackapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) makeRequest(ctx context.Context, method string, path string, values url.Values) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, method, c.baseUrl+path, nil)
	req.URL.RawQuery = values.Encode()
	if c.token != "" {
		req.Header.Add("Authorization", "Bearer "+c.token)
	}
	req.Header.Add("Accept", "application/json")

	return req
}

func (c *Client) makeRequestWithBody(ctx context.Context, method string, path string, values url.Values, body []byte) *http.Request {
	r := c.makeRequest(ctx, method, path, values)
	r.Header.Set("Content-Type", "application/json")
	r.Body = io.NopCloser(bytes.NewReader(body))
	return r
}
