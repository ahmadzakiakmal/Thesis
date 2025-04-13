package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Configuration options
var (
	listenAddr     = flag.String("listen", ":7980", "Address to listen on for Layer 2 requests")
	layer1Endpoint = flag.String("layer1", "http://localhost:5000", "Layer 1 API endpoint")
	nodeID         = flag.String("node-id", "l2-adapter-node", "Node ID for this adapter")
)

// Layer 1 compatible structures
type DeWSRequest struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	RemoteAddr string            `json:"remote_addr"`
	RequestID  string            `json:"request_id"`
	Timestamp  time.Time         `json:"timestamp"`
}

type DeWSResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
	BodyCustom interface{}       `json:"body_custom"`
}

type DeWSTransaction struct {
	Request      DeWSRequest  `json:"request"`
	Response     DeWSResponse `json:"response"`
	OriginNodeID string       `json:"origin_node_id"`
	BlockHeight  int64        `json:"block_height,omitempty"`
}

// JSONRPC structures for minimal Rollkit compatibility
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

func main() {
	flag.Parse()

	// Initialize logger
	logger := log.New(os.Stdout, "L2-ADAPTER: ", log.LstdFlags)
	logger.Printf("Starting DeWS Layer 2 Adapter on %s", *listenAddr)
	logger.Printf("Layer 1 endpoint: %s", *layer1Endpoint)

	// Set up HTTP server with routing
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/forward", handleForward)

	server := &http.Server{
		Addr:    *listenAddr,
		Handler: mux,
	}

	// Start the server
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	logger.Println("Server started successfully")

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Println("Server exited properly")
}

// handleRoot handles the root endpoint
func handleRoot(w http.ResponseWriter, r *http.Request) {
	// Special case for JSONRPC requests
	if r.Method == "POST" && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		fmt.Println("JSONRPC request received")
		fmt.Println(r.Method)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		fmt.Println(string(body))
		// Restore body for further processing
		r.Body = io.NopCloser(strings.NewReader(string(body)))

		// Try to parse as JSONRPC
		var req JSONRPCRequest
		if err := json.Unmarshal(body, &req); err == nil && req.Method != "" {
			handleJSONRPC(w, req)
			return
		}
	}

	// Default handler for non-JSONRPC requests
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h1>DeWS Layer 2 Adapter</h1>")
	fmt.Fprintf(w, "<p>Node ID: %s</p>", *nodeID)
	fmt.Fprintf(w, "<p>Layer 1 Endpoint: %s</p>", *layer1Endpoint)
	fmt.Fprintf(w, "<p>Use /forward endpoint to forward requests to Layer 1</p>")
}

// handleForward forwards requests to Layer 1
func handleForward(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Generate a request ID
	requestID := generateRequestID()

	// Extract headers
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	// Create a DeWSRequest
	dewsReq := DeWSRequest{
		Method:     r.Method,
		Path:       r.URL.Query().Get("path"), // Get actual path from query parameter
		Headers:    headers,
		Body:       string(body),
		RemoteAddr: r.RemoteAddr,
		RequestID:  requestID,
		Timestamp:  time.Now(),
	}

	// If path is empty, use a default
	if dewsReq.Path == "" {
		dewsReq.Path = "/api/customers" // Default path
	}

	// Create a DeWSTransaction
	dewsTx := DeWSTransaction{
		Request:      dewsReq,
		OriginNodeID: *nodeID,
	}

	// Forward to Layer 1
	fmt.Println(dewsTx)
	response, err := forwardToLayer1(dewsTx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error forwarding to Layer 1: %v", err), http.StatusInternalServerError)
		return
	}

	// Set response headers
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}
	w.Header().Set("X-L2-Request-ID", requestID)
	w.WriteHeader(response.StatusCode)

	// Write response body
	w.Write([]byte(response.Body))
}

// handleJSONRPC handles JSONRPC requests for Rollkit compatibility
func handleJSONRPC(w http.ResponseWriter, req JSONRPCRequest) {
	// Implement minimal JSONRPC methods for Rollkit compatibility
	switch req.Method {
	case "da.MaxBlobSize":
		// Return maximum blob size
		sendJSONRPCResponse(w, req.ID, 1024*1024) // 1MB
	case "da.Submit":
		// Handle minimal DA submit
		txID := generateRequestID()
		idBytes := []byte(txID)
		sendJSONRPCResponse(w, req.ID, [][]byte{idBytes})
	case "status":
		// Return minimal system status
		status := map[string]interface{}{
			"node_info": map[string]interface{}{
				"network": "layer2-rollup",
				"node_id": *nodeID,
			},
		}
		sendJSONRPCResponse(w, req.ID, status)
	default:
		sendJSONRPCErrorResponse(w, req.ID, -32601, "Method not found")
	}
}

// forwardToLayer1 forwards a transaction to Layer 1
func forwardToLayer1(tx DeWSTransaction) (*DeWSResponse, error) {
	// Create the Layer 1 URL from the transaction path
	layer1URL := *layer1Endpoint + tx.Request.Path

	// Create a new HTTP request
	req, err := http.NewRequest(tx.Request.Method, layer1URL, strings.NewReader(tx.Request.Body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	for key, value := range tx.Request.Headers {
		req.Header.Set(key, value)
	}

	// Send the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Extract headers
	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}

	// Create response
	dewsResp := &DeWSResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       string(respBody),
	}

	return dewsResp, nil
}

// sendJSONRPCResponse sends a successful JSONRPC response
func sendJSONRPCResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendJSONRPCErrorResponse sends an error JSONRPC response
func sendJSONRPCErrorResponse(w http.ResponseWriter, id interface{}, code int, message string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
		},
		ID: id,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s-%d", *nodeID, time.Now().UnixNano())))
	return hex.EncodeToString(h.Sum(nil))
}
