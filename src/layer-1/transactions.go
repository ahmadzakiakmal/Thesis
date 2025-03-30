package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DeWSRequest represents the client's original HTTP request
type DeWSRequest struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	RemoteAddr string            `json:"remote_addr"`
	RequestID  string            `json:"request_id"` // Unique ID for the request
	Timestamp  time.Time         `json:"timestamp"`
}

// DeWSResponse represents the computed response from a server
type DeWSResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// DeWSTransaction represents a complete DeWS transaction
type DeWSTransaction struct {
	Request      DeWSRequest  `json:"request"`
	Response     DeWSResponse `json:"response"`
	OriginNodeID string       `json:"origin_node_id"` // ID of the node that originated the transaction
	BlockHeight  int64        `json:"block_height,omitempty"`
}

// Add this struct to your transaction types
type DeWSTx struct {
	ServerInfo    string      `json:"serverInfo"`
	RequestURL    string      `json:"requestURL"`
	RequestMethod string      `json:"requestMethod"`
	APIInfo       string      `json:"apiInfo"`
	RequestTime   time.Time   `json:"requestTime"`
	RequestData   interface{} `json:"requestData"`
	ResponseData  interface{} `json:"responseData"`
}

// SerializeToBytes converts the transaction to a byte array for blockchain storage
func (t *DeWSTransaction) SerializeToBytes() ([]byte, error) {
	return json.Marshal(t)
}

// DeserializeFromBytes converts byte array from blockchain back to transaction
func DeserializeFromBytes(data []byte) (*DeWSTransaction, error) {
	var tx DeWSTransaction
	err := json.Unmarshal(data, &tx)
	return &tx, err
}

// ConvertHTTPRequestToDeWSRequest converts an http.Request to DeWSRequest
func ConvertHTTPRequestToDeWSRequest(r *http.Request, requestID string) (*DeWSRequest, error) {
	// Extract headers
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	// Read body if present
	body := ""
	if r.Body != nil {
		// Limited to reasonable size to prevent attacks
		bodyBytes := make([]byte, 10*1024*1024) // 10MB limit
		n, _ := r.Body.Read(bodyBytes)
		if n > 0 {
			body = string(bodyBytes[:n])
		}
	}

	return &DeWSRequest{
		Method:     r.Method,
		Path:       r.URL.Path,
		Headers:    headers,
		Body:       body,
		RemoteAddr: r.RemoteAddr,
		RequestID:  requestID,
		Timestamp:  time.Now(),
	}, nil
}

// GenerateResponse executes the request and generates a response
func (req *DeWSRequest) GenerateResponse(services *ServiceRegistry) (*DeWSResponse, error) {
	// This is where we'll implement the actual service execution
	// For now, it's a placeholder

	// Find the appropriate service handler for this request
	handler, found := services.GetHandlerForPath(req.Method, req.Path)
	if !found {
		return &DeWSResponse{
			StatusCode: http.StatusNotFound,
			Headers:    map[string]string{"Content-Type": "text/plain"},
			Body:       fmt.Sprintf("Service not found for %s %s", req.Method, req.Path),
		}, nil
	}

	// Execute the handler
	return handler(req)
}
