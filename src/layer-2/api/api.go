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
	"syscall"
	"time"
)

// Configuration options
var (
	httpPort      = flag.String("http-port", "5100", "HTTP web server port")
	sequencerPort = flag.String("sequencer-port", "26657", "Sequencer port for broadcast_tx_sync")
	nodeID        = flag.String("node-id", "l2-node", "Node ID for this node")
)

// DeWSRequest represents a request in the DeWS protocol
type DeWSRequest struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	RemoteAddr string            `json:"remote_addr"`
	RequestID  string            `json:"request_id"`
	Timestamp  time.Time         `json:"timestamp"`
}

// DeWSResponse represents a response in the DeWS protocol
type DeWSResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// DeWSTransaction represents a complete transaction in the DeWS protocol
type DeWSTransaction struct {
	Request      DeWSRequest  `json:"request"`
	Response     DeWSResponse `json:"response"`
	OriginNodeID string       `json:"origin_node_id"`
	BlockHeight  int64        `json:"block_height,omitempty"`
}

func main() {
	// Parse command line flags
	flag.Parse()

	// Set up logger
	logger := log.New(os.Stdout, "LAYER2: ", log.LstdFlags)
	logger.Println("Starting simple Layer 2 node...")
	logger.Printf("Sequencer port: %s\n", *sequencerPort)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/debug", handleDebug)
	mux.HandleFunc("/api/customers", handleCustomer)

	server := &http.Server{
		Addr:    ":" + *httpPort,
		Handler: mux,
	}

	// Start the server
	go func() {
		logger.Printf("Starting HTTP server on port %s", *httpPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	logger.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Server shutdown error: %v", err)
	}
	logger.Println("Server stopped")
}

// handleRoot displays basic information about the node
func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h1>DeWS Layer 2 Node</h1>")
	fmt.Fprintf(w, "<p>Node ID: %s</p>", *nodeID)
	fmt.Fprintf(w, "<p>API endpoint: /api/customers</p>")
	fmt.Fprintf(w, "<p>Debug endpoint: /debug</p>")
}

// handleDebug provides debug information
func handleDebug(w http.ResponseWriter, r *http.Request) {
	debugInfo := map[string]interface{}{
		"node_id":        *nodeID,
		"server_time":    time.Now(),
		"sequencer_port": *sequencerPort,
		"http_port":      *httpPort,
		"api_version":    "1.0",
		"runtime_since":  time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

// handleCustomer handles the /api/customers endpoint
func handleCustomer(w http.ResponseWriter, r *http.Request) {
	// Only handle POST requests
	if r.Method != "POST" {
		http.Error(w, "Method not allowed. Only POST is supported.", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// Parse the customer data
	var customer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := json.Unmarshal(body, &customer); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Validate the customer data
	if customer.Name == "" || customer.Email == "" {
		http.Error(w, "Name and email are required", http.StatusBadRequest)
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

	// Create DeWSRequest
	dewsReq := DeWSRequest{
		Method:     r.Method,
		Path:       r.URL.Path,
		Headers:    headers,
		Body:       string(body),
		RemoteAddr: r.RemoteAddr,
		RequestID:  requestID,
		Timestamp:  time.Now(),
	}

	// Create a response that indicates the transaction is pending confirmation
	dewsResp := DeWSResponse{
		StatusCode: http.StatusAccepted, // 202 Accepted instead of 201 Created
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"message":"Request validated and submitted to consensus","id":"%s"}`, requestID),
	}

	// Create DeWSTransaction
	dewsTx := DeWSTransaction{
		Request:      dewsReq,
		Response:     dewsResp,
		OriginNodeID: *nodeID,
	}

	// Send to sequencer
	txStatus, err := sendToSequencer(dewsTx)
	if err != nil {
		log.Printf("Error sending to sequencer: %v", err)
		http.Error(w, "Error processing request", http.StatusInternalServerError)
		return
	}

	// Create response with transaction info
	response := map[string]interface{}{
		"message":    "Request validated and submitted to consensus",
		"request_id": requestID,
		"status":     "pending_confirmation",
		"tx_hash":    txStatus, // This will now be the actual transaction hash
		"timestamp":  time.Now(),
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Transaction-Status", txStatus)
	w.WriteHeader(http.StatusAccepted) // 202 Accepted

	// Write response
	json.NewEncoder(w).Encode(response)
}

// sendToSequencer sends the transaction to the sequencer's broadcast_tx_sync endpoint
func sendToSequencer(tx DeWSTransaction) (string, error) {
	// Serialize the transaction to JSON
	txData, err := json.Marshal(tx)
	if err != nil {
		return "", fmt.Errorf("error serializing transaction: %v", err)
	}

	// Create direct HTTP request to CometBFT endpoint
	txHex := hex.EncodeToString(txData)
	sequencerURL := fmt.Sprintf("http://localhost:%s/broadcast_tx_sync?tx=%s", *sequencerPort, txHex)

	// Log the request for debugging
	log.Printf("Sending transaction with length %d bytes via direct endpoint", len(txData))
	log.Printf("URL: %s", sequencerURL)

	// Make GET request to the endpoint
	resp, err := http.Get(sequencerURL)
	if err != nil {
		return "", fmt.Errorf("error sending to sequencer: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	// Log the raw response for debugging
	log.Printf("Sequencer response: %s", string(respBody))

	// Parse response
	var responseMap map[string]interface{}
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	// Check for error
	if errObj, hasError := responseMap["error"]; hasError && errObj != nil {
		return "failed", fmt.Errorf("RPC error: %v", errObj)
	}

	// Try to extract the transaction hash from the response
	if result, ok := responseMap["result"].(map[string]interface{}); ok {
		if hash, ok := result["hash"].(string); ok {
			// Return the actual transaction hash
			return hash, nil
		}
	}

	// Fallback if we couldn't extract the hash
	return "validation_successful", nil
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	// Create a unique hash based on time and node ID
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s-%d", *nodeID, time.Now().UnixNano())))
	return hex.EncodeToString(h.Sum(nil))
}
