package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	cmtlog "github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/rpc/client"
	cmthttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/google/uuid"
)

// DeWSWebServer handles HTTP requests using the DeWS protocol
type DeWSWebServer struct {
	app              *DeWSApplication
	httpAddr         string
	server           *http.Server
	logger           cmtlog.Logger
	node             *nm.Node
	startTime        time.Time
	serviceRegistry  *ServiceRegistry
	tendermintClient client.Client
	peers            map[string]string // nodeID -> RPC URL
	peersMu          sync.RWMutex
}

// TransactionStatus represents the consensus status of a transaction
type TransactionStatus struct {
	TxID          string        `json:"tx_id"`
	RequestID     string        `json:"request_id"`
	Status        string        `json:"status"`
	BlockHeight   int64         `json:"block_height"`
	BlockHash     string        `json:"block_hash,omitempty"`
	ConfirmTime   time.Time     `json:"confirm_time,omitempty"`
	ResponseInfo  ResponseInfo  `json:"response_info,omitempty"`
	ConsensusInfo ConsensusInfo `json:"consensus_info,omitempty"`
}

// ResponseInfo contains information about the response
type ResponseInfo struct {
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type,omitempty"`
	BodyLength  int    `json:"body_length"`
}

// ConsensusInfo contains information about the consensus process
type ConsensusInfo struct {
	TotalNodes     int    `json:"total_nodes"`
	AgreementNodes int    `json:"agreement_nodes"`
	NodeResponses  []bool `json:"node_responses,omitempty"`
}

// ClientResponse is the response format sent to clients
type ClientResponse struct {
	StatusCode    int               `json:"-"` // Not included in JSON
	Headers       map[string]string `json:"-"` // Not included in JSON
	Body          string            `json:"body,omitempty"`
	Meta          TransactionStatus `json:"meta"`
	BlockchainRef string            `json:"blockchain_ref"`
	NodeID        string            `json:"node_id"`
}

// NewDeWSWebServer creates a new DeWS web server
func NewDeWSWebServer(app *DeWSApplication, httpPort string, logger cmtlog.Logger, node *nm.Node, serviceRegistry *ServiceRegistry) (*DeWSWebServer, error) {
	mux := http.NewServeMux()

	rpcAddr := fmt.Sprintf("http://localhost:%s", extractPortFromAddress(node.Config().RPC.ListenAddress))
	logger.Info("Connecting to CometBFT RPC", "address", rpcAddr)

	// Create HTTP client without WebSocket
	tendermintClient, err := cmthttp.NewWithClient(
		rpcAddr,
		&http.Client{
			Timeout: 10 * time.Second,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CometBFT client: %w", err)
	}
	err = tendermintClient.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start CometBFT client: %w", err)
	}

	server := &DeWSWebServer{
		app:      app,
		httpAddr: ":" + httpPort,
		server: &http.Server{
			Addr:    ":" + httpPort,
			Handler: mux,
		},
		logger:           logger,
		node:             node,
		startTime:        time.Now(),
		serviceRegistry:  serviceRegistry,
		tendermintClient: tendermintClient,
		peers:            make(map[string]string),
	}

	// Register routes
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/debug", server.handleDebug)
	mux.HandleFunc("/api/", server.handleAPI)
	mux.HandleFunc("/status/", server.handleTransactionStatus)

	// Discover peers (in a production system, this would be more sophisticated)
	server.discoverPeers()

	return server, nil
}

// Start starts the web server
func (server *DeWSWebServer) Start() error {
	server.logger.Info("Starting DeWS web server", "addr", server.httpAddr)
	go func() {
		if err := server.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			server.logger.Error("DeWS web server error: ", "err", err)
		}
	}()
	return nil
}

// Shutdown gracefully shuts down the web server
func (server *DeWSWebServer) Shutdown(ctx context.Context) error {
	server.logger.Info("Shutting down DeWS web server")
	return server.server.Shutdown(ctx)
}

// discoverPeers discovers other nodes in the network
func (server *DeWSWebServer) discoverPeers() {
	// For now, we'll use a simple approach based on the persistent_peers from config
	// In a production system, this would be more dynamic

	persistentPeers := server.node.Config().P2P.PersistentPeers
	if persistentPeers == "" {
		server.logger.Info("No persistent peers configured")
		return
	}

	peers := strings.Split(persistentPeers, ",")
	for _, peer := range peers {
		parts := strings.Split(peer, "@")
		if len(parts) != 2 {
			continue
		}

		nodeID := parts[0]
		address := parts[1]

		// Extract host and port
		hostPort := strings.Split(address, ":")
		if len(hostPort) != 2 {
			continue
		}

		host := hostPort[0]
		p2pPort := hostPort[1]

		// Calculate RPC port (this is an assumption, might need to be adjusted)
		// In our setup script, p2p ports are even (9000, 9002, etc.) and RPC ports are odd (9001, 9003, etc.)
		p2pPortInt, err := strconv.Atoi(p2pPort)
		if err != nil {
			continue
		}

		rpcPort := strconv.Itoa(p2pPortInt + 1)
		rpcURL := fmt.Sprintf("http://%s:%s", host, rpcPort)

		server.peersMu.Lock()
		server.peers[nodeID] = rpcURL
		server.peersMu.Unlock()

		server.logger.Info("Discovered peer", "nodeID", nodeID, "rpcURL", rpcURL)
	}
}

func (server *DeWSWebServer) directBroadcastTxCommit(txData string) (map[string]interface{}, error) {
	// Base64 encode the transaction
	txBase64 := base64.StdEncoding.EncodeToString([]byte(txData))

	// Create JSON-RPC request
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "broadcast_tx_commit",
		"params": map[string]interface{}{
			"tx": txBase64, // Send as base64 encoded
		},
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the exact request being sent
	server.logger.Info("Sending RPC request", "body", string(requestJSON))

	// Send the request
	rpcAddr := fmt.Sprintf("http://localhost:%s", extractPortFromAddress(server.node.Config().RPC.ListenAddress))
	resp, err := http.Post(rpcAddr, "application/json", bytes.NewReader(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read and log the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	server.logger.Info("RPC response", "status", resp.Status, "body", string(respBody))

	// Parse the response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// Add this function to your DeWSWebServer struct
func (server *DeWSWebServer) processAPIRequest(method, path string, data interface{}) (interface{}, error) {
	// This is a simplified version - you'll need to expand this based on your API needs
	switch {
	case method == "GET" && path == "customers":
		// Get all customers
		// In a real implementation, this would query your database
		return []map[string]interface{}{
			{"id": 1, "name": "John Doe", "email": "john@example.com", "address": "123 Main St"},
			{"id": 2, "name": "Jane Smith", "email": "jane@example.com", "address": "456 Oak Ave"},
		}, nil

	case method == "POST" && path == "customers":
		// Create a new customer
		// In a real implementation, this would insert into your database
		customerData, ok := data.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid customer data format")
		}

		// Mock customer creation with a new ID
		customerData["id"] = time.Now().Unix()
		return customerData, nil

	default:
		return nil, fmt.Errorf("unsupported API endpoint: %s %s", method, path)
	}
}

// handleRoot handles the root endpoint which shows node status
func (server *DeWSWebServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	// nodeTemplate, err := template.ParseFiles("templates/node.tmpl")
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	// Similar to original server.go, display node status
	// You can reuse the NodeData structure and related code
	// ...

	w.Write([]byte("<h1>DeWS Node</h1>"))
	w.Write([]byte("<p>Node ID: " + string(server.node.NodeInfo().ID()) + "</p>"))
	w.Write([]byte("<p>This node implements the DeWS protocol (Decentralized and Byzantine Fault-tolerant Web Services)</p>"))
}

// handleDebug provides debugging information
func (server *DeWSWebServer) handleDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Collect debug information
	nodeStatus := "online"
	if server.node.ConsensusReactor().WaitSync() {
		nodeStatus = "syncing"
	}
	if !server.node.IsListening() {
		nodeStatus = "offline"
	}
	debugInfo := map[string]interface{}{
		"node_id":     string(server.node.NodeInfo().ID()),
		"node_status": nodeStatus,
		"p2p_address": server.node.Config().P2P.ListenAddress,
		"rpc_address": server.node.Config().RPC.ListenAddress,
		"uptime":      time.Since(server.startTime).String(),
	}

	// Get Tendermint status
	status, err := server.tendermintClient.Status(context.Background())
	outboundPeers, inboundPeers, dialingPeers := server.node.Switch().NumPeers()
	debugInfo["num_peers_out"] = outboundPeers
	debugInfo["num_peers_in"] = inboundPeers
	debugInfo["num_peers_dialing"] = dialingPeers
	if err != nil {
		debugInfo["tendermint_error"] = err.Error()
	} else {
		debugInfo["node_status"] = "online"
		debugInfo["latest_block_height"] = status.SyncInfo.LatestBlockHeight
		debugInfo["latest_block_time"] = status.SyncInfo.LatestBlockTime
		debugInfo["catching_up"] = status.SyncInfo.CatchingUp

		peers := make([]map[string]interface{}, 0, len(server.node.Switch().Peers().Copy()))
		debugInfo["peers"] = peers
	}

	// Add ABCI info
	abciInfo, err := server.tendermintClient.ABCIInfo(context.Background())
	if err != nil {
		debugInfo["abci_error"] = err.Error()
	} else {
		debugInfo["abci_version"] = abciInfo.Response.Version
		debugInfo["app_version"] = abciInfo.Response.AppVersion
		debugInfo["last_block_height"] = abciInfo.Response.LastBlockHeight
		debugInfo["last_block_app_hash"] = fmt.Sprintf("%X", abciInfo.Response.LastBlockAppHash)
	}

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(debugInfo); err != nil {
		http.Error(w, "Error encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleAPI handles API requests using the DeWS protocol
func (server *DeWSWebServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Check if consensus should be used
	consensusParam := r.URL.Query().Get("consensus")
	withConsensus := consensusParam != "0" // Default to using consensus

	// Generate a unique request ID
	requestID, err := generateRequestID()
	fmt.Println(requestID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		server.logger.Error("Failed to generate request ID", "err", err)
		return
	}

	// Parse and process the request locally
	var requestData interface{}
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		if err := json.Unmarshal(body, &requestData); err != nil {
			http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
			return
		}
	}

	// Extract path without the /api/ prefix
	apiPath := strings.TrimPrefix(r.URL.Path, "/api/")

	// Generate response locally (similar to what the original code does)
	apiResponse, err := server.processAPIRequest(r.Method, apiPath, requestData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		server.logger.Error("Failed to process API request", "err", err)
		return
	}

	localProcessTime := time.Since(startTime)
	server.logger.Info("Local processing time", "duration", localProcessTime)

	// If consensus is disabled, return response immediately
	if !withConsensus {
		server.logger.Info("Skipping consensus", "path", r.URL.Path)
		response := map[string]interface{}{
			"webserviceData": apiResponse,
			"consensusData":  nil,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create transaction for consensus
	id := uuid.NewString()
	tx := map[string]string{
		"method":      r.Method,
		"path":        apiPath,
		"timestamp":   time.Now().Format(time.RFC3339),
		"transaction": "Create User",
		"id":          id,
	}
	server.logger.Info("Tx Received", tx["method"], tx["path"], tx["timestamp"])

	// Convert map to JSON string
	txJSON, err := json.Marshal(tx)
	if err != nil {
		http.Error(w, "Failed to serialize transaction", http.StatusInternalServerError)
		server.logger.Error("Failed to serialize transaction", "err", err)
		return
	}
	txString := string(txJSON)

	// Broadcast transaction and wait for commitment
	consensusStart := time.Now()
	tendermintResponse, err := server.directBroadcastTxCommit(txString)
	consensusTime := time.Since(consensusStart)
	server.logger.Info("Consensus time", "duration", consensusTime)

	if err != nil {
		http.Error(w, "Consensus error: "+err.Error(), http.StatusInternalServerError)
		server.logger.Error("Failed to broadcast transaction", "err", err)
		return
	}

	// Parse height from response
	var blockHeight string
	result, ok := tendermintResponse["result"].(map[string]interface{})
	fmt.Println("=== TENDERMINT RESPONSE ===")
	fmt.Println(result)
	if ok {
		blockHeight, _ = result["height"].(string)
	}

	// Create response with consensus data
	response := map[string]interface{}{
		"webserviceData": apiResponse,
		"consensusData": map[string]interface{}{
			"height": blockHeight,
		},
	}

	// Return the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	totalTime := time.Since(startTime)
	server.logger.Info("Total request time",
		"path", r.URL.Path,
		"method", r.Method,
		"withConsensus", withConsensus,
		"duration", totalTime)
}

// handleTransactionStatus returns the status of a transaction
func (server *DeWSWebServer) handleTransactionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract transaction ID from URL
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) != 3 || pathParts[1] != "status" {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	txID := pathParts[2]

	// Check transaction status
	status, err := server.checkTransactionStatus(txID)
	if err != nil {
		http.Error(w, "Error checking transaction status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if status == nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Return status as JSON
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(status)
	if err != nil {
		http.Error(w, "Error encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// checkTransactionStatus checks the status of a transaction in the blockchain
func (server *DeWSWebServer) checkTransactionStatus(txID string) (*TransactionStatus, error) {
	// Query the blockchain for the transaction
	query := fmt.Sprintf("tx.hash='%s'", txID)
	res, err := server.tendermintClient.TxSearch(context.Background(), query, false, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("error searching for transaction: %w", err)
	}

	if len(res.Txs) == 0 {
		return nil, nil // Transaction not found
	}

	tx := res.Txs[0]

	// Parse the transaction
	var dewsTx DeWSTransaction
	err = json.Unmarshal(tx.Tx, &dewsTx)
	if err != nil {
		return nil, fmt.Errorf("error parsing transaction: %w", err)
	}

	// Extract events
	status := "pending"
	for _, event := range tx.TxResult.Events {
		if event.Type == "dews_tx" {
			for _, attr := range event.Attributes {
				if string(attr.Key) == "status" {
					status = string(attr.Value)
				}
			}
		}
	}

	// Create response info
	responseInfo := ResponseInfo{
		StatusCode:  dewsTx.Response.StatusCode,
		ContentType: dewsTx.Response.Headers["Content-Type"],
		BodyLength:  len(dewsTx.Response.Body),
	}

	// Create consensus info (in a real system, this would be more detailed)
	consensusInfo := ConsensusInfo{
		TotalNodes:     server.countPeers() + 1, // +1 for this node
		AgreementNodes: 1,                       // Simplified for now
	}

	// Create transaction status
	txStatus := &TransactionStatus{
		TxID:          txID,
		RequestID:     dewsTx.Request.RequestID,
		Status:        status,
		BlockHeight:   tx.Height,
		BlockHash:     fmt.Sprintf("%X", tx.Hash),
		ConfirmTime:   time.Unix(0, time.Now().Unix()), // TODO
		ResponseInfo:  responseInfo,
		ConsensusInfo: consensusInfo,
	}

	return txStatus, nil
}

// countPeers counts the number of peers
func (server *DeWSWebServer) countPeers() int {
	server.peersMu.RLock()
	defer server.peersMu.RUnlock()
	return len(server.peers)
}

// Helper Functions

// generateRequestID generates a unique request ID
func generateRequestID() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// extractPortFromAddress extracts the port from an address string
func extractPortFromAddress(address string) string {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == ':' {
			return address[i+1:]
		}
	}
	return ""
}
