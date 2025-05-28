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

type RequestOptions struct {
	Headers         map[string]string
	Timeout         time.Duration
	FollowRedirects bool
	Context         context.Context
}

type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Error      error
}

type HTTPClient struct {
	BaseURL     string
	Client      *http.Client
	DefaultOpts RequestOptions
}

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

func (c *HTTPClient) Call(method, endpoint string, body interface{}, opts *RequestOptions) (*Response, error) {

	if opts == nil {
		opts = &c.DefaultOpts
	}

	url := c.BaseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return &Response{Error: err}, err
		}
		bodyReader = bytes.NewBuffer(bodyJSON)
	}

	ctx := opts.Context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return &Response{Error: err}, err
	}

	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if !opts.FollowRedirects {
		c.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		c.Client.CheckRedirect = nil
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return &Response{Error: err}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Response{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Error:      err,
		}, err
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
		Error:      nil,
	}, nil
}

func (c *HTTPClient) GET(endpoint string, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodGet, endpoint, nil, opts)
}

func (c *HTTPClient) POST(endpoint string, body interface{}, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodPost, endpoint, body, opts)
}

func (c *HTTPClient) PUT(endpoint string, body interface{}, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodPut, endpoint, body, opts)
}

func (c *HTTPClient) PATCH(endpoint string, body interface{}, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodPatch, endpoint, body, opts)
}

func (c *HTTPClient) DELETE(endpoint string, opts *RequestOptions) (*Response, error) {
	return c.Call(http.MethodDelete, endpoint, nil, opts)
}

func UnmarshalBody(resp *Response, target interface{}) error {

	if len(resp.Body) == 0 {
		return fmt.Errorf("empty response body")
	}

	err := json.Unmarshal(resp.Body, target)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return nil
}
