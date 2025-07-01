package srvreg

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository"
	cmtlog "github.com/cometbft/cometbft/libs/log"
)

// Request represents the client's original HTTP request
type Request struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	RemoteAddr string            `json:"remote_addr"`
	RequestID  string            `json:"request_id"` // Unique ID for the request
	Timestamp  time.Time         `json:"timestamp"`
}

// GenerateRequestID generates a deterministic ID for the request
func (r *Request) GenerateRequestID() {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s-%s-%s-%s", r.Path, r.Method, r.Body, r.Timestamp)))
	r.RequestID = hex.EncodeToString(hasher.Sum(nil)[:16])
}

// Response represents the computed response from a server
type Response struct {
	StatusCode    int               `json:"status_code"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	Error         string            `json:"error,omitempty"`
	BodyInterface interface{}       `json:"body_interface"`
}

// ParseBody attempts to parse the Response's Body field as JSON
// and returns the structured data or nil if parsing fails.
func (r *Response) ParseBody() interface{} {
	// If Body is empty, return nil
	if r.Body == "" {
		return nil
	}

	// First try to unmarshal into a map (JSON object)
	var bodyMap map[string]interface{}
	err := json.Unmarshal([]byte(r.Body), &bodyMap)
	if err == nil {
		return bodyMap
	}

	// If that fails, try as a JSON array
	var bodyArray []interface{}
	err = json.Unmarshal([]byte(r.Body), &bodyArray)
	if err == nil {
		log.Printf("Successful body parse")
		return bodyArray
	}

	// If not valid JSON, return nil
	log.Println("Invalid JSON")
	log.Println(err)
	return nil
}

// Transaction represents a complete transaction, pairing the Request and the Response
type Transaction struct {
	Request      Request  `json:"request"`
	Response     Response `json:"response"`
	OriginNodeID string   `json:"origin_node_id"` // ID of the node that originated the transaction
	BlockHeight  int64    `json:"block_height,omitempty"`
}

// ServiceHandler is a function type for service handlers
type ServiceHandler func(*Request) (*Response, error)

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
	repository  *repository.Repository
	logger      cmtlog.Logger
	isByzantine bool
}

// SerializeToBytes converts the transaction to a byte array for blockchain storage
func (t *Transaction) SerializeToBytes() ([]byte, error) {
	return json.Marshal(t)
}

// ConvertHttpRequestToConsensusRequest converts an http.Request to Request
func ConvertHttpRequestToConsensusRequest(r *http.Request, requestID string) (*Request, error) {
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
		// bodyBytes := make([]byte, 10*1024*1024) // 10MB limit to prevent attacks
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		raw := strings.TrimSpace(string(bodyBytes))
		body = compactJSON(raw)
	}

	return &Request{
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
func NewServiceRegistry(
	repository *repository.Repository,
	logger cmtlog.Logger,
	isByzantine bool,
) *ServiceRegistry {
	return &ServiceRegistry{
		handlers:    make(map[RouteKey]ServiceHandler),
		exactRoutes: make(map[RouteKey]bool),
		repository:  repository,
		logger:      logger,
		isByzantine: isByzantine,
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

// GetHandlerForPath finds the appropriate handler for a given path and a boolean of whether or not the handler was found
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

// matchPath does simple pattern matching for routes.
// It supports patterns like "/user/:id" matching "/user/123"
func matchPath(pattern, path string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i := range len(patternParts) {
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

// RegisterDefaultServices sets up
// the default services for the BFT system
func (sr *ServiceRegistry) RegisterDefaultServices() {
	// Endpoints
	// Test Create Package Endpoint
	sr.RegisterHandler(
		"POST",
		"/session/test-package",
		true,
		sr.CreateTestPackage,
	)
	// Create Session Endpoint
	sr.RegisterHandler(
		"POST",
		"/session/start",
		true,
		sr.CreateSessionHandler,
	)
	// Scan Package Ednpoint
	sr.RegisterHandler(
		"GET",
		"/session/:id/scan/:packageID",
		false,
		sr.ScanPackageHandler,
	)
	// Validate Package Endpoint
	sr.RegisterHandler(
		"POST",
		"/session/:id/validate",
		false,
		sr.ValidatePackageHandler,
	)
	// Quality Check Endpoint
	sr.RegisterHandler(
		"POST",
		"/session/:id/qc",
		false,
		sr.QualityCheckHandler,
	)
	// Label Package Endpoint
	sr.RegisterHandler(
		"POST",
		"/session/:id/label",
		false,
		sr.LabelPackageHandler,
	)
	// Commit Session Endpoint
	sr.RegisterHandler(
		"POST",
		"/commit/:id",
		false,
		sr.CommitSessionHandler,
	)
}

// GenerateResponse executes the request and generates a response
func (req *Request) GenerateResponse(services *ServiceRegistry) (*Response, error) {
	// Find the appropriate service handler for this request
	handler, found := services.GetHandlerForPath(req.Method, req.Path)
	log.Println("matching service registry handler...")
	if !found {
		log.Println("service registry handler not found")
		return &Response{
			StatusCode: http.StatusNotFound,
			Headers:    map[string]string{"Content-Type": "text/plain"},
			Body:       fmt.Sprintf("Service not found for %s %s", req.Method, req.Path),
		}, nil
	}
	log.Println("service registry found")

	// Execute the handler
	response, err := handler(req)

	if services.isByzantine {
		if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusCreated {
			response.Body = `{"message": "Byzantiner node response - data corrupted"}`
			response.StatusCode = http.StatusInternalServerError
		}
		services.logger.Info("Byzantine Node Response", response.Body)
	}

	return response, err
}

func compactJSON(body string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(body)); err != nil {
		// If it’s not JSON, return trimmed original
		return strings.TrimSpace(body)
	}
	return buf.String()
}
