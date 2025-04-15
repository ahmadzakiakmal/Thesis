package service_registry

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"encoding/json"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-1/server/models"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"gorm.io/gorm"
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

// Response represents the computed response from a server
type Response struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
	BodyCustom interface{}       `json:"body_custom"`
}

// Transaction represents a complete transaction, pairing the Request and the Response
type Transaction struct {
	Request      Request  `json:"request"`
	Response     Response `json:"response"`
	OriginNodeID string   `json:"origin_node_id"` // ID of the node that originated the transaction
	BlockHeight  int64    `json:"block_height,omitempty"`
}

// Add this struct to your transaction types
// type DeWSTx struct {
// 	ServerInfo    string      `json:"serverInfo"`
// 	RequestURL    string      `json:"requestURL"`
// 	RequestMethod string      `json:"requestMethod"`
// 	APIInfo       string      `json:"apiInfo"`
// 	RequestTime   time.Time   `json:"requestTime"`
// 	RequestData   interface{} `json:"requestData"`
// 	ResponseData  interface{} `json:"responseData"`
// }

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
	database    *gorm.DB
	logger      cmtlog.Logger
	isByzantine bool
}

// SerializeToBytes converts the transaction to a byte array for blockchain storage
func (t *Transaction) SerializeToBytes() ([]byte, error) {
	return json.Marshal(t)
}

// ConvertHttpRequestToRequest converts an http.Request to Request
func ConvertHttpRequestToRequest(r *http.Request, requestID string) (*Request, error) {
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
func NewServiceRegistry(db *gorm.DB, logger cmtlog.Logger, isByzantine bool) *ServiceRegistry {
	return &ServiceRegistry{
		handlers:    make(map[RouteKey]ServiceHandler),
		exactRoutes: make(map[RouteKey]bool),
		database:    db,
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

// RegisterDefaultServices sets up the default services for the BFT system
func (sr *ServiceRegistry) RegisterDefaultServices() {
	sr.RegisterHandler("POST", "/api/customers", true, func(req *Request) (*Response, error) {
		var newUser models.User
		err := json.Unmarshal([]byte(req.Body), &newUser)
		if err != nil {
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       "failed to parse body",
			}, fmt.Errorf("failed to parse body")
		}
		dbTx := sr.database.Begin()
		err = dbTx.Create(&models.User{
			Name:  newUser.Name,
			Email: newUser.Email,
		}).Error
		log.Println("=========")
		log.Printf("Registering User with email :%s\n", newUser.Email)
		log.Println("=========")
		if err != nil {
			dbTx.Rollback()
			log.Printf("Error on DB transaction: %s\n", err.Error())
			responseBody := fmt.Sprintf("error on database transaction: %s", err.Error())
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       responseBody,
			}, fmt.Errorf("error on database transaction: %s", err.Error())
		}

		log.Println("Success")
		dbTx.Commit()
		return &Response{
			StatusCode: http.StatusCreated,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       fmt.Sprintf(`{"message":"Customer created successfully","email":"%s","requestId":"%s"}`, newUser.Email, req.RequestID),
		}, nil
	})

	sr.RegisterHandler("GET", "/api/customers", true, func(req *Request) (*Response, error) {
		var customers []models.User
		err := sr.database.Find(&customers).Error
		if err != nil {
			responseBody := fmt.Sprintf("error on database transaction: %s", err.Error())
			return &Response{
				StatusCode: http.StatusUnprocessableEntity,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       responseBody,
			}, fmt.Errorf("error on database transaction: %s", err.Error())
		}

		responseBody := map[string]interface{}{
			"customers": customers,
		}
		jsonBytes, err := json.Marshal(responseBody)
		if err != nil {
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error":"failed to marshal response"}`,
			}, err
		}
		return &Response{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(jsonBytes),
			BodyCustom: customers,
		}, nil
	})

	// Additional routes can be added here
}

// GenerateResponse executes the request and generates a response
func (req *Request) GenerateResponse(services *ServiceRegistry) (*Response, error) {
	// Find the appropriate service handler for this request
	handler, found := services.GetHandlerForPath(req.Method, req.Path)
	if !found {
		return &Response{
			StatusCode: http.StatusNotFound,
			Headers:    map[string]string{"Content-Type": "text/plain"},
			Body:       fmt.Sprintf("Service not found for %s %s", req.Method, req.Path),
		}, nil
	}

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
