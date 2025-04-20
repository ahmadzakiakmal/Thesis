package srvreg

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository"
	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository/models"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/lib/pq"
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
		return bodyArray
	}

	// If not valid JSON, return nil
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
	database    *gorm.DB
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
	// The original DeWS endpoints, for baseline testing only
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
			StatusCode:    http.StatusOK,
			Headers:       map[string]string{"Content-Type": "application/json"},
			Body:          string(jsonBytes),
			BodyInterface: customers,
		}, nil
	})

	// Endpoints

	type startSessionBody struct {
		OperatorID string `json:"operator_id"`
	}
	// Create Session Endpoint
	sr.RegisterHandler("POST", "/session/start", true, func(req *Request) (*Response, error) {
		sessionID := fmt.Sprintf("SESSION-%s", req.RequestID)
		// operatorID := "OPR-001" // TODO: get from request
		var body startSessionBody
		err := json.Unmarshal([]byte(req.Body), &body)
		if err != nil {
			sr.logger.Info("Failed to parse body", "error", err.Error())
			return &Response{
				StatusCode: http.StatusUnprocessableEntity,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error":"Failed to create session: %s"}`, err.Error()),
			}, err
		}

		operatorID := body.OperatorID
		if operatorID == "" {
			return &Response{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error":"operator ID is required"}`,
			}, err
		}
		session := models.Session{
			ID:          sessionID,
			Status:      "active",
			IsCommitted: false,
			OperatorID:  operatorID,
		}

		dbTx := sr.database.Begin()
		err = dbTx.Create(&session).Error
		if err != nil {
			dbTx.Rollback()
			if pqErr, ok := err.(*pq.Error); ok {
				switch pqErr.Code {
				case repository.PgErrForeignKeyViolation: // foreign_key_violation
					return &Response{
						StatusCode: http.StatusBadRequest,
						Headers:    map[string]string{"Content-Type": "application/json"},
						Body:       fmt.Sprintf(`{"error":"Invalid reference: %s"}`, pqErr.Detail),
					}, fmt.Errorf("foreign key violation: %v", pqErr.Detail)

				case repository.PgErrUniqueViolation: // unique_violation
					return &Response{
						StatusCode: http.StatusConflict,
						Headers:    map[string]string{"Content-Type": "application/json"},
						Body:       fmt.Sprintf(`{"error":"Record already exists: %s"}`, pqErr.Detail),
					}, fmt.Errorf("unique violation: %v", pqErr.Detail)

				default:
					return &Response{
						StatusCode: http.StatusInternalServerError,
						Headers:    map[string]string{"Content-Type": "application/json"},
						Body:       fmt.Sprintf(`{"error":"Database error: %s (Code: %s)"}`, pqErr.Message, pqErr.Code),
					}, fmt.Errorf("database error: %v", err)
				}
			}
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error":"Failed to create session: %s"}`, err.Error()),
			}, err
		}

		err = dbTx.Commit().Error
		if err != nil {
			sr.logger.Info("Failed to commit session transaction", "error", err.Error())
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error":"Failed to create session: %s"}`, err.Error()),
			}, err
		}

		return &Response{
			StatusCode: http.StatusCreated, // or http.StatusOK
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       fmt.Sprintf(`{"message":"Session generated","id":"%s"}`, sessionID),
		}, nil
	})

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
