package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RequestOptions allows customizing the HTTP request
type RequestOptions struct {
	Headers         map[string]string
	Timeout         time.Duration
	FollowRedirects bool
	Context         context.Context
}

// Response represents the HTTP response
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Error      error
}

// HTTPClient is a customizable HTTP client for making requests
type HTTPClient struct {
	BaseURL     string
	Client      *http.Client
	DefaultOpts RequestOptions
}

// NewHTTPClient creates a new HTTPClient with default configuration
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		DefaultOpts: RequestOptions{
			Headers:         map[string]string{},
			Timeout:         30 * time.Second,
			FollowRedirects: true,
			Context:         context.Background(),
		},
	}
}

// Call performs an HTTP request with the specified method, endpoint, and body
func (c *HTTPClient) Call(method, endpoint string, body interface{}, opts *RequestOptions) (*Response, error) {
	// Use default options if none provided
	if opts == nil {
		opts = &c.DefaultOpts
	}

	// Prepare URL
	url := c.BaseURL + endpoint

	// Prepare request body
	var bodyReader io.Reader
	if body != nil {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return &Response{Error: err}, err
		}
		bodyReader = bytes.NewBuffer(bodyJSON)
	}

	// Create context with timeout
	ctx := opts.Context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return &Response{Error: err}, err
	}

	// Set headers
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// Set content type if body is present
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Configure redirect policy
	if !opts.FollowRedirects {
		c.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		c.Client.CheckRedirect = nil
	}

	// Send request
	resp, err := c.Client.Do(req)
	if err != nil {
		return &Response{Error: err}, err
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Response{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Error:      err,
		}, err
	}

	// Return response
	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
		Error:      nil,
	}, nil
}

// GET is a convenience method for making GET requests
func (c *HTTPClient) GET(endpoint string, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodGet, endpoint, nil, opts)
}

// POST is a convenience method for making POST requests
func (c *HTTPClient) POST(endpoint string, body interface{}, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodPost, endpoint, body, opts)
}

// PUT is a convenience method for making PUT requests
func (c *HTTPClient) PUT(endpoint string, body interface{}, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodPut, endpoint, body, opts)
}

// PATCH is a convenience method for making PATCH requests
func (c *HTTPClient) PATCH(endpoint string, body interface{}, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodPatch, endpoint, body, opts)
}

// DELETE is a convenience method for making DELETE requests
func (c *HTTPClient) DELETE(endpoint string, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodDelete, endpoint, nil, opts)
}

// UnmarshalBody reads the response body and unmarshals it into the provided struct
func UnmarshalBody(resp *Response, target interface{}) error {
	// Check if body is empty
	if len(resp.Body) == 0 {
		return fmt.Errorf("empty response body")
	}

	// Unmarshal the JSON body into the target struct
	err := json.Unmarshal(resp.Body, target)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return nil
}
