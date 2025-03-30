package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

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
