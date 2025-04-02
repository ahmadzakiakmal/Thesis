package service_registry

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"encoding/json"
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

// ServiceHandler is a function type for service handlers
type ServiceHandler func(*DeWSRequest) (*DeWSResponse, error)

// RouteKey is used to uniquely identify a route
type RouteKey struct {
	Method string
	Path   string
}

// ServiceRegistry manages all service handlers
type ServiceRegistry struct {
	handlers    map[RouteKey]ServiceHandler
	exactRoutes map[RouteKey]bool // Whether a route is exact or pattern-based
	mu          sync.RWMutex
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

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		handlers:    make(map[RouteKey]ServiceHandler),
		exactRoutes: make(map[RouteKey]bool),
	}
}

// RegisterHandler registers a new service handler
func (sr *ServiceRegistry) RegisterHandler(method, path string, isExactPath bool, handler ServiceHandler) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	key := RouteKey{Method: strings.ToUpper(method), Path: path}
	sr.handlers[key] = handler
	sr.exactRoutes[key] = isExactPath
}

// GetHandlerForPath finds the appropriate handler for a given path
func (sr *ServiceRegistry) GetHandlerForPath(method, path string) (ServiceHandler, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	// Try exact match first
	key := RouteKey{Method: strings.ToUpper(method), Path: path}
	if handler, ok := sr.handlers[key]; ok {
		if sr.exactRoutes[key] {
			return handler, true
		}
	}

	// Try pattern matching
	for routeKey, handler := range sr.handlers {
		if routeKey.Method != strings.ToUpper(method) {
			continue
		}

		// Skip exact routes in pattern matching
		if sr.exactRoutes[routeKey] {
			continue
		}

		// Simple pattern matching - can be enhanced
		if matchPath(routeKey.Path, path) {
			return handler, true
		}
	}

	return nil, false
}

// matchPath does simple pattern matching for routes
// It supports patterns like "/user/:id" matching "/user/123"
func matchPath(pattern, path string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i := 0; i < len(patternParts); i++ {
		if strings.HasPrefix(patternParts[i], ":") {
			// This is a parameter part, it matches anything
			continue
		}

		if patternParts[i] != pathParts[i] {
			return false
		}
	}

	return true
}

// RegisterDefaultServices sets up the default services for the DeWS system
func (sr *ServiceRegistry) RegisterDefaultServices() {
	// Example: Register a customer service
	sr.RegisterHandler("POST", "/api/customers", true, func(req *DeWSRequest) (*DeWSResponse, error) {
		// In a real implementation, this would create a customer in a database
		return &DeWSResponse{
			StatusCode: http.StatusCreated,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       fmt.Sprintf(`{"message":"Customer created successfully","id":"customer-123","requestId":"%s"}`, req.RequestID),
		}, nil
	})

	sr.RegisterHandler("GET", "/api/customers", true, func(req *DeWSRequest) (*DeWSResponse, error) {
		// In a real implementation, this would get customers from a database
		return &DeWSResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"customers":[{"id":"customer-123","name":"Example Customer"}]}`,
		}, nil
	})

	// Additional routes can be added here
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
