package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Configuration options
var (
	listenAddr     = flag.String("listen", ":7980", "Address to listen on for Layer 2 requests")
	layer1Endpoint = flag.String("layer1", "http://localhost:5000", "Layer 1 API endpoint")
	daMode         = flag.String("da-mode", "mock", "Data availability mode: 'mock' or 'celestia'")
	nodeID         = flag.String("node-id", "l2-adapter-node", "Node ID for this adapter")
	logLevel       = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	batchInterval  = flag.Duration("batch-interval", 5*time.Second, "Time interval for batching transactions")
	batchSize      = flag.Int("batch-size", 100, "Maximum number of transactions in a batch")
)

// Transaction represents a transaction in the system
type Transaction struct {
	ID        string          `json:"id"`
	Request   json.RawMessage `json:"request"`
	Response  json.RawMessage `json:"response,omitempty"`
	Status    string          `json:"status"`
	Timestamp time.Time       `json:"timestamp"`
	BlockID   uint64          `json:"block_id,omitempty"`
}

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

// JSONRPC request structure
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

// JSONRPC response structure
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// BatchProcessor handles the batching of transactions
type BatchProcessor struct {
	transactions      []*Transaction
	mu                sync.Mutex
	batchInterval     time.Duration
	batchSize         int
	lastBatchTime     time.Time
	layer1Endpoint    string
	currentBlockID    uint64
	processingBatch   bool
	processingBatchMu sync.Mutex
}

// TransactionStore is a simple in-memory store for transactions
type TransactionStore struct {
	transactions map[string]*Transaction
	mu           sync.RWMutex
}

// Global variables
var (
	processor *BatchProcessor
	txStore   *TransactionStore
	logger    *log.Logger
)

func main() {
	flag.Parse()

	// Initialize logger
	logger = log.New(os.Stdout, "L2-ADAPTER: ", log.LstdFlags|log.Lshortfile)
	logger.Printf("Starting DeWS Layer 2 Adapter on %s", *listenAddr)
	logger.Printf("Layer 1 endpoint: %s", *layer1Endpoint)
	logger.Printf("Data availability mode: %s", *daMode)

	// Initialize transaction store
	txStore = &TransactionStore{
		transactions: make(map[string]*Transaction),
	}

	// Initialize batch processor
	processor = &BatchProcessor{
		transactions:   make([]*Transaction, 0),
		batchInterval:  *batchInterval,
		batchSize:      *batchSize,
		lastBatchTime:  time.Now(),
		layer1Endpoint: *layer1Endpoint,
		currentBlockID: 1, // Start from block 1
	}

	// Start the batch processor
	go processor.processBatches()

	// Set up HTTP server with routing
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/tx", handleTransaction)
	mux.HandleFunc("/status/", handleStatus)
	mux.HandleFunc("/batch", handleBatch)

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

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Println("Shutting down server...")

	// Process any remaining transactions
	processor.processBatchNow()

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
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}

		// Restore body for further processing
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Try to parse as JSONRPC
		var req JSONRPCRequest
		if err := json.Unmarshal(body, &req); err == nil && req.Method != "" {
			handleJSONRPC(w, req, body)
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
	fmt.Fprintf(w, "<p>Current batch size: %d transactions</p>", len(processor.transactions))
	fmt.Fprintf(w, "<p>Current block ID: %d</p>", processor.currentBlockID)
}

// handleTransaction handles transaction submissions
func handleTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Parse the transaction
	var tx Transaction
	if err := json.Unmarshal(body, &tx); err != nil {
		http.Error(w, "Invalid transaction format", http.StatusBadRequest)
		return
	}

	// Generate a transaction ID if not provided
	if tx.ID == "" {
		tx.ID = generateTxID()
	}

	// Set timestamp if not provided
	if tx.Timestamp.IsZero() {
		tx.Timestamp = time.Now()
	}

	// Set status to "pending"
	tx.Status = "pending"

	// Store the transaction
	txStore.add(&tx)

	// Add to batch processor
	processor.addTransaction(&tx)

	// Return the transaction ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":     tx.ID,
		"status": tx.Status,
	})
}

// handleStatus checks the status of a transaction
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract transaction ID from URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 || parts[1] != "status" {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	txID := parts[2]
	tx := txStore.get(txID)
	if tx == nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Return the transaction status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tx)
}

// handleBatch handles batch operations
func handleBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Process the current batch
	processor.processBatchNow()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Batch processing initiated",
	})
}

// handleJSONRPC handles JSONRPC requests
func handleJSONRPC(w http.ResponseWriter, req JSONRPCRequest, rawBody []byte) {
	logger.Printf("Handling JSONRPC: %s", req.Method)

	// Handle different methods
	switch req.Method {
	case "da.MaxBlobSize":
		// Return maximum blob size
		sendJSONRPCResponse(w, req.ID, 1024*1024) // 1MB

	case "da.Submit":
		// Handle DA submit - we assume the params contain transaction data
		// For simplicity, let's create a transaction from it
		tx := &Transaction{
			ID:        generateTxID(),
			Request:   rawBody,
			Status:    "pending",
			Timestamp: time.Now(),
		}

		// Store the transaction
		txStore.add(tx)

		// Add to batch
		processor.addTransaction(tx)

		// Return the transaction ID as bytes
		idBytes := []byte(tx.ID)
		sendJSONRPCResponse(w, req.ID, [][]byte{idBytes})

	case "status":
		// Return system status
		status := map[string]interface{}{
			"node_info": map[string]interface{}{
				"network": "layer2-rollup",
				"node_id": *nodeID,
			},
			"sync_info": map[string]interface{}{
				"latest_block_height": processor.currentBlockID,
			},
			"batch_info": map[string]interface{}{
				"current_batch_size": len(processor.transactions),
				"last_batch_time":    processor.lastBatchTime,
			},
		}
		sendJSONRPCResponse(w, req.ID, status)

	case "da.GetIDs":
		// Parse the block height from params
		var height uint64
		if err := json.Unmarshal(req.Params, &height); err != nil {
			sendJSONRPCErrorResponse(w, req.ID, -32602, "Invalid params")
			return
		}

		// Get all transaction IDs for this block height
		var ids []string
		txStore.mu.RLock()
		for _, tx := range txStore.transactions {
			if tx.BlockID == height {
				ids = append(ids, tx.ID)
			}
		}
		txStore.mu.RUnlock()

		// Convert to bytes
		idBytes := make([][]byte, len(ids))
		for i, id := range ids {
			idBytes[i] = []byte(id)
		}

		sendJSONRPCResponse(w, req.ID, map[string]interface{}{
			"ids": idBytes,
		})

	case "da.Get":
		// Parse the IDs from params
		var idBytes [][]byte
		if err := json.Unmarshal(req.Params, &idBytes); err != nil {
			sendJSONRPCErrorResponse(w, req.ID, -32602, "Invalid params")
			return
		}

		// Get transactions for these IDs
		var txs []json.RawMessage
		for _, id := range idBytes {
			tx := txStore.get(string(id))
			if tx != nil {
				txs = append(txs, tx.Request)
			}
		}

		sendJSONRPCResponse(w, req.ID, txs)

	default:
		sendJSONRPCErrorResponse(w, req.ID, -32601, "Method not found")
	}
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

// addTransaction adds a transaction to the batch
func (p *BatchProcessor) addTransaction(tx *Transaction) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.transactions = append(p.transactions, tx)
	logger.Printf("Added transaction %s to batch. Current batch size: %d", tx.ID, len(p.transactions))

	// If we've reached the batch size, process immediately
	if len(p.transactions) >= p.batchSize {
		go p.processBatchNow()
	}
}

// processBatches periodically processes batches
func (p *BatchProcessor) processBatches() {
	ticker := time.NewTicker(p.batchInterval)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		if time.Since(p.lastBatchTime) >= p.batchInterval && len(p.transactions) > 0 {
			p.mu.Unlock()
			p.processBatchNow()
		} else {
			p.mu.Unlock()
		}
	}
}

// processBatchNow processes the current batch immediately
func (p *BatchProcessor) processBatchNow() {
	// Use a mutex to ensure only one batch is being processed at a time
	p.processingBatchMu.Lock()
	defer p.processingBatchMu.Unlock()

	p.mu.Lock()
	if len(p.transactions) == 0 {
		p.mu.Unlock()
		return
	}

	// Get the current batch
	currentBatch := p.transactions
	p.transactions = make([]*Transaction, 0)
	p.lastBatchTime = time.Now()
	blockID := p.currentBlockID
	p.currentBlockID++
	p.mu.Unlock()

	logger.Printf("Processing batch of %d transactions for block %d", len(currentBatch), blockID)

	// Convert to Layer 1 format and submit
	for _, tx := range currentBatch {
		// Update block ID
		tx.BlockID = blockID

		// Log the transaction content if it's not empty
		if tx.Request != nil && len(tx.Request) > 0 {
			// Print a line of equals to make it easier to find in logs
			logger.Println("================ TRANSACTION TO LAYER 1 ================")

			// Log transaction ID and block
			logger.Printf("TX ID: %s, Block ID: %d", tx.ID, blockID)

			// Try to pretty print the JSON body
			var requestObj interface{}
			if err := json.Unmarshal(tx.Request, &requestObj); err == nil {
				prettyJSON, err := json.MarshalIndent(requestObj, "", "  ")
				if err == nil {
					logger.Printf("Request (JSON):\n%s", string(prettyJSON))
				} else {
					logger.Printf("Request: %s", string(tx.Request))
				}
			} else {
				// If not valid JSON, log as string
				logger.Printf("Request: %s", string(tx.Request))
			}

			// Print a closing line
			logger.Println("================ END TRANSACTION ================")
		}

		// Convert to Layer 1 format and submit
		err := p.submitToLayer1(tx)
		if err != nil {
			tx.Status = "failed"
			logger.Printf("Failed to submit transaction %s: %v", tx.ID, err)
		} else {
			tx.Status = "confirmed"
		}

		// Update in store
		txStore.update(tx)
	}

	logger.Printf("Batch processing complete for block %d", blockID)
}

// submitToLayer1 submits a transaction to Layer 1 using the RPC endpoint
func (p *BatchProcessor) submitToLayer1(tx *Transaction) error {
	// Parse the raw request
	var rawRequest map[string]interface{}
	if err := json.Unmarshal(tx.Request, &rawRequest); err != nil {
		return fmt.Errorf("invalid request format: %v", err)
	}

	// Extract needed fields to create DeWSRequest
	method, _ := rawRequest["method"].(string)
	path, _ := rawRequest["path"].(string)
	body, _ := rawRequest["body"].(string)

	// Create DeWSRequest
	dewsReq := DeWSRequest{
		Method:     method,
		Path:       path,
		Headers:    make(map[string]string),
		Body:       body,
		RemoteAddr: "layer2-adapter",
		RequestID:  tx.ID,
		Timestamp:  tx.Timestamp,
	}

	// Add headers if provided
	if headersMap, ok := rawRequest["headers"].(map[string]interface{}); ok {
		for k, v := range headersMap {
			if strValue, ok := v.(string); ok {
				dewsReq.Headers[k] = strValue
			}
		}
	}

	// Convert to Layer 1 format for submission
	dewsTransaction := DeWSTransaction{
		Request:      dewsReq,
		OriginNodeID: *nodeID,
	}

	// Serialize the transaction to JSON bytes
	txData, err := json.Marshal(dewsTransaction)
	if err != nil {
		return fmt.Errorf("error serializing transaction: %v", err)
	}

	// For CometBFT RPC, we need to hex encode the raw bytes
	encodedTx := hex.EncodeToString(txData)

	// Construct RPC endpoint URL - using port 9001 as specified
	rpcURL := getRPCEndpoint(p.layer1Endpoint)

	// Create broadcast_tx_sync JSONRPC request
	rpcRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "broadcast_tx_sync",
		"params":  []string{encodedTx},
	}

	rpcRequestJSON, err := json.Marshal(rpcRequest)
	if err != nil {
		return fmt.Errorf("error serializing RPC request: %v", err)
	}

	// Send RPC request
	logger.Printf("Submitting transaction to Layer 1 RPC endpoint: %s", rpcURL)
	req, err := http.NewRequest("POST", rpcURL, bytes.NewBuffer(rpcRequestJSON))
	if err != nil {
		return fmt.Errorf("error creating RPC request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending RPC request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading RPC response: %v", err)
	}

	// Parse the RPC response
	var rpcResponse struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Code int    `json:"code"`
			Data string `json:"data"`
			Hash string `json:"hash"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(respBody, &rpcResponse); err != nil {
		return fmt.Errorf("error parsing RPC response: %v", err)
	}

	// Check for RPC errors
	if rpcResponse.Error != nil {
		return fmt.Errorf("RPC error: %s (code %d)", rpcResponse.Error.Message, rpcResponse.Error.Code)
	}

	// Check for transaction errors
	if rpcResponse.Result.Code != 0 {
		return fmt.Errorf("transaction error: code %d", rpcResponse.Result.Code)
	}

	// Store the transaction hash
	hash := rpcResponse.Result.Hash
	logger.Printf("Transaction %s submitted to Layer 1 with hash: %s", tx.ID, hash)

	// Create a simulated response for now
	// In a real implementation, we would either wait for the transaction to be committed
	// or retrieve the result from a query
	simulatedResponse := DeWSResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"success":true,"tx_hash":"%s","block_height":0}`, hash),
	}

	// Serialize and store response
	respData, err := json.Marshal(simulatedResponse)
	if err != nil {
		return fmt.Errorf("error serializing response: %v", err)
	}

	tx.Response = respData

	return nil
}

// getRPCEndpoint determines the CometBFT RPC endpoint based on the Layer 1 API endpoint
func getRPCEndpoint(apiEndpoint string) string {
	// If the endpoint explicitly contains an RPC path, use it directly
	if strings.Contains(apiEndpoint, "/rpc") {
		return apiEndpoint
	}

	// Extract the host and port
	url, err := url.Parse(apiEndpoint)
	if err != nil {
		// If parsing fails, make best effort - assume default port 9001
		logger.Printf("Error parsing endpoint URL: %v, using localhost:9001", err)
		return "http://localhost:9001"
	}

	host := url.Hostname()

	// Use port 9001 as requested
	return fmt.Sprintf("http://%s:9001", host)
}

// add adds a transaction to the store
func (s *TransactionStore) add(tx *Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[tx.ID] = tx
}

// get retrieves a transaction from the store
func (s *TransactionStore) get(id string) *Transaction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.transactions[id]
}

// update updates a transaction in the store
func (s *TransactionStore) update(tx *Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[tx.ID] = tx
}

// generateTxID generates a unique transaction ID
func generateTxID() string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", *nodeID, time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])
}
