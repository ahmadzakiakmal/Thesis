package main

import (
	"context"
	"crypto/rand"
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
	debugInfo["num_peers_dialingn"] = dialingPeers
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
	// Generate a unique request ID
	requestID, err := generateRequestID()
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		server.logger.Error("Failed to generate request ID", "err", err)
		return
	}

	// Convert HTTP request to DeWS request
	dewsRequest, err := ConvertHTTPRequestToDeWSRequest(r, requestID)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		server.logger.Error("Failed to convert HTTP request", "err", err)
		return
	}

	if r.Method == http.MethodGet {
		// Execute request locally
		dewsResponse, err := dewsRequest.GenerateResponse(server.serviceRegistry)
		if err != nil {
			// Handle error
		}

		// Respond directly without blockchain
		w.Header().Set("Content-Type", dewsResponse.Headers["Content-Type"])
		w.WriteHeader(dewsResponse.StatusCode)
		io.WriteString(w, dewsResponse.Body)
		return
	}

	// Generate response locally
	dewsResponse, err := dewsRequest.GenerateResponse(server.serviceRegistry)
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		server.logger.Error("Failed to generate response", "err", err)
		return
	}

	// Create transaction
	tx := DeWSTransaction{
		Request:      *dewsRequest,
		Response:     *dewsResponse,
		OriginNodeID: string(server.node.NodeInfo().ID()),
	}

	// Submit transaction to blockchain
	txBytes, err := tx.SerializeToBytes()
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		server.logger.Error("Failed to serialize transaction", "err", err)
		return
	}

	// Broadcast transaction to blockchain
	result, err := server.tendermintClient.BroadcastTxSync(context.Background(), txBytes)
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		server.logger.Error("Failed to broadcast transaction", "err", err)
		return
	}

	// Check if transaction was accepted
	if result.Code != 0 {
		http.Error(w, "Transaction Rejected: "+result.Log, http.StatusBadRequest)
		server.logger.Error("Transaction rejected", "code", result.Code, "log", result.Log)
		return
	}

	// Create transaction ID
	txID := generateTxID(requestID, string(server.node.NodeInfo().ID()))

	// Wait for transaction to be included in a block
	maxWaitTime := 10 * time.Second
	interval := 100 * time.Millisecond
	deadline := time.Now().Add(maxWaitTime)

	var txStatus *TransactionStatus

	for time.Now().Before(deadline) {
		status, err := server.checkTransactionStatus(txID)
		if err == nil && status != nil {
			txStatus = status
			break
		}

		time.Sleep(interval)
	}

	if txStatus == nil {
		// We didn't get confirmation, but the transaction was accepted
		// Return a response anyway, but note that it's not confirmed
		txStatus = &TransactionStatus{
			TxID:      txID,
			RequestID: requestID,
			Status:    "pending",
		}
	}

	// Create client response
	clientResponse := ClientResponse{
		StatusCode:    dewsResponse.StatusCode,
		Headers:       dewsResponse.Headers,
		Body:          dewsResponse.Body,
		Meta:          *txStatus,
		BlockchainRef: fmt.Sprintf("/status/%s", txID),
		NodeID:        string(server.node.NodeInfo().ID()),
	}

	// Set response headers
	for key, value := range clientResponse.Headers {
		w.Header().Set(key, value)
	}

	// Add DeWS-specific headers
	w.Header().Set("DeWS-Transaction-ID", txID)
	w.Header().Set("DeWS-Status", txStatus.Status)
	if txStatus.BlockHeight > 0 {
		w.Header().Set("DeWS-Block-Height", fmt.Sprintf("%d", txStatus.BlockHeight))
	}

	// Set status code
	w.WriteHeader(clientResponse.StatusCode)

	// Write response body
	io.WriteString(w, clientResponse.Body)
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
