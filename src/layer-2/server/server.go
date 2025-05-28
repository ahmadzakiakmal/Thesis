package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-2/app"
	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository"
	service_registry "github.com/ahmadzakiakmal/thesis/src/layer-2/srvreg"

	cmtlog "github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/rpc/client"
	cmthttp "github.com/cometbft/cometbft/rpc/client/http"
	cmtrpc "github.com/cometbft/cometbft/rpc/client/local"
)

// WebServer handles HTTP requests
type WebServer struct {
	app                *app.Application
	httpAddr           string
	server             *http.Server
	logger             cmtlog.Logger
	node               *nm.Node
	startTime          time.Time
	serviceRegistry    *service_registry.ServiceRegistry
	cometBftHttpClient client.Client
	cometBftRpcClient  *cmtrpc.Local
	peers              map[string]string // nodeID -> RPC URL
	repository         *repository.Repository
}

// TransactionStatus represents the consensus status of a transaction
type TransactionStatus struct {
	TxID         string         `json:"tx_id"`
	RequestID    string         `json:"request_id"`
	Status       string         `json:"status"`
	BlockHeight  int64          `json:"block_height"`
	BlockHash    string         `json:"block_hash,omitempty"`
	ConfirmTime  time.Time      `json:"confirm_time"`
	ResponseInfo ResponseInfo   `json:"response_info"`
	BlockTxs     BlockTxsDetail `json:"block_txs"`
}

// BlockTxsDetail contains the transactions within a block
type BlockTxsDetail struct {
	BlockTransactions    []service_registry.Transaction `json:"block_transactions"`
	BlockTransactionsB64 []string                       `json:"block_transactions_b64"`
}

// ResponseInfo contains information about the response
type ResponseInfo struct {
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type,omitempty"`
	BodyLength  int    `json:"body_length"`
}

// ConsensusInfo contains information about the consensus process
type ConsensusInfo struct {
	TotalNodes     int           `json:"total_nodes"`
	AgreementNodes int           `json:"agreement_nodes"`
	NodeResponses  []bool        `json:"node_responses,omitempty"`
	Votes          []interface{} `json:"votes"`
}

// ClientResponse is the response format sent to clients
type ClientResponse struct {
	StatusCode int               `json:"-"` // Not included in JSON
	Headers    map[string]string `json:"-"` // Not included in JSON
	// Body          string            `json:"body,omitempty"`
	Body          interface{}       `json:"body"`
	Meta          TransactionStatus `json:"meta"`
	BlockchainRef string            `json:"blockchain_ref"`
	NodeID        string            `json:"node_id"`
}

// NewWebServer creates a new web server
func NewWebServer(app *app.Application, httpPort string, logger cmtlog.Logger, node *nm.Node, serviceRegistry *service_registry.ServiceRegistry, repository *repository.Repository) (*WebServer, error) {
	mux := http.NewServeMux()

	rpcAddr := fmt.Sprintf("http://localhost:%s", extractPortFromAddress(node.Config().RPC.ListenAddress))
	logger.Info("Connecting to CometBFT RPC", "address", rpcAddr)

	// Create HTTP client without WebSocket
	cometBftHttpClient, err := cmthttp.NewWithClient(
		rpcAddr,
		&http.Client{
			Timeout: 10 * time.Second,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CometBFT client: %w", err)
	}
	err = cometBftHttpClient.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start CometBFT client: %w", err)
	}

	server := &WebServer{
		app:      app,
		httpAddr: ":" + httpPort,
		server: &http.Server{
			Addr:    ":" + httpPort,
			Handler: mux,
		},
		logger:             logger,
		node:               node,
		startTime:          time.Now(),
		serviceRegistry:    serviceRegistry,
		cometBftHttpClient: cometBftHttpClient,
		cometBftRpcClient:  cmtrpc.New(node),
		peers:              make(map[string]string),
		repository:         repository,
	}

	// Register routes
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/debug", server.handleDebug)
	mux.HandleFunc("/status/", server.handleTransactionStatus)
	// Session Endpoints
	mux.HandleFunc("/session/", server.handleSessionAPI)
	mux.HandleFunc("/commit/", server.handleCommitAPI)

	return server, nil
}

// Start starts the web server
func (ws *WebServer) Start() error {
	ws.logger.Info("Starting web server", "addr", ws.httpAddr)
	go func() {
		if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ws.logger.Error("web server error: ", "err", err)
		}
	}()
	return nil
}

// Shutdown gracefully shuts down the web server
func (ws *WebServer) Shutdown(ctx context.Context) error {
	ws.logger.Info("Shutting down web server")
	return ws.server.Shutdown(ctx)
}

// handleRoot handles the root endpoint which shows node status
func (ws *WebServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	w.Write([]byte("<h1>Session Aware Consensus Simulator Node</h1>"))
	w.Write([]byte("<p>Node ID: " + string(ws.node.NodeInfo().ID()) + "</p>"))
	rpcPort := extractPortFromAddress(ws.node.Config().RPC.ListenAddress)
	rpcAddrHtml := fmt.Sprintf("<p>RPC Address: <a href=\"http://localhost:%s\">http://localhost:%s</a>", rpcPort, rpcPort)
	w.Write([]byte(rpcAddrHtml))
}

// handleDebug provides debugging information
func (ws *WebServer) handleDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Collect debug information
	nodeStatus := "online"
	if ws.node.ConsensusReactor().WaitSync() {
		nodeStatus = "syncing"
	}
	if !ws.node.IsListening() {
		nodeStatus = "offline"
	}

	debugInfo := map[string]interface{}{
		"node_id":     string(ws.node.NodeInfo().ID()),
		"node_status": nodeStatus,
		"p2p_address": ws.node.Config().P2P.ListenAddress,
		"rpc_address": ws.node.Config().RPC.ListenAddress,
		"uptime":      time.Since(ws.startTime).String(),
	}

	// Get Tendermint status
	status, err := ws.cometBftRpcClient.Status(context.Background())
	outboundPeers, inboundPeers, dialingPeers := ws.node.Switch().NumPeers()
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

		peers := make([]map[string]interface{}, 0, len(ws.node.Switch().Peers().Copy()))
		debugInfo["peers"] = peers
	}

	// Add ABCI info
	abciInfo, err := ws.cometBftRpcClient.ABCIInfo(context.Background())
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
		JSONError(w, "Error encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleTransactionStatus returns the status of a transaction
func (ws *WebServer) handleTransactionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract transaction ID from URL
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) != 3 || pathParts[1] != "status" {
		JSONError(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	txID := pathParts[2]

	// Check transaction status
	status, err := ws.checkTransactionStatus(txID)
	if err != nil {
		JSONError(w, "Error checking transaction status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if status == nil {
		JSONError(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Return status as JSON
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(status)
	if err != nil {
		JSONError(w, "Error encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleSessionAPI handels API requests regarding supply chain session
func (ws *WebServer) handleSessionAPI(w http.ResponseWriter, r *http.Request) {
	requestID, err := generateRequestID()
	if err != nil {
		JSONError(w, "Internal Server Error", http.StatusInternalServerError)
		ws.logger.Error("Failed to generate request ID", "err", err)
		return
	}

	request, err := service_registry.ConvertHttpRequestToConsensusRequest(r, requestID)
	if err != nil {
		JSONError(w, "Failed to convert request: "+err.Error(), http.StatusUnprocessableEntity)
		ws.logger.Error("Failed to convert HTTP request", "err", err)
		return
	}
	// request.Body = strings.TrimSpace(request.Body)

	response, err := request.GenerateResponse(ws.serviceRegistry)
	if err != nil {
		JSONError(w, "Failed to generate response: "+err.Error(), http.StatusUnprocessableEntity)
		ws.logger.Error("Failed to generate response", "err", err)
		return
	}

	transaction := &service_registry.Transaction{
		Request:      *request,
		Response:     *response,
		OriginNodeID: string(ws.node.ConsensusReactor().Switch.NodeInfo().ID()),
	}

	// Simulate transaction consensus
	consensusResponse, repoErr := ws.repository.RunConsensus(context.Background(), transaction)
	if repoErr != nil {
		if repoErr.Code == "CONSENSUS_ERROR" {
			JSONError(w, "Consensus error occurred: "+repoErr.Detail, http.StatusInternalServerError)
			return
		}
		JSONError(w, "An error occured: "+repoErr.Message, http.StatusInternalServerError)
	}

	// Add block height info to consensus transaction
	blockHeight := consensusResponse.BlockHeight
	transaction.BlockHeight = blockHeight

	block, err := ws.cometBftRpcClient.Block(context.Background(), &blockHeight)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if block.Block == nil {
		ws.logger.Info("Web Server", "Block not found")
	}

	var blockTransactionsB64 []string
	var blockTransactions []service_registry.Transaction
	for _, tx := range block.Block.Txs {
		// Convert the transaction bytes to base64
		b64Tx := base64.StdEncoding.EncodeToString(tx)
		blockTransactionsB64 = append(blockTransactionsB64, b64Tx)

		// Try to parse the transaction
		var parsedTx service_registry.Transaction
		if err := json.Unmarshal(tx, &parsedTx); err == nil {
			parsedTx.Response.BodyInterface = parsedTx.Response.ParseBody()
			blockTransactions = append(blockTransactions, parsedTx)
		} else {
			// If parsing fails, you might want to log the error
			ws.logger.Error("Failed to parse transaction", "err", err)
		}
	}

	// Respond to client
	apiResponse := ClientResponse{
		StatusCode: response.StatusCode,
		Headers:    response.Headers,
		Body:       response.ParseBody(),
		Meta: TransactionStatus{
			TxID:        consensusResponse.TxHash,
			RequestID:   requestID,
			Status:      "confirmed",
			BlockHeight: blockHeight,
			// BlockHash:   hex.EncodeToString(consensusResponse.Hash),
			ConfirmTime: time.Now(),
			ResponseInfo: ResponseInfo{
				StatusCode:  response.StatusCode,
				ContentType: response.Headers["Content-Type"],
				BodyLength:  len(response.Body),
			},
			BlockTxs: BlockTxsDetail{
				BlockTransactions:    blockTransactions,
				BlockTransactionsB64: blockTransactionsB64,
			},
		},
		NodeID: transaction.OriginNodeID,
	}

	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(apiResponse)
	if err != nil {
		ws.logger.Error("Failed to encode client response", "err", err)
	}

	ws.logger.Info("=== Req-Res Pair Result ===",
		transaction.Request.Path,
		transaction.Request.Method,
		transaction.Request.Body,
		transaction.Response.StatusCode,
		transaction.Response.Body,
	)
}

func (ws *WebServer) handleCommitAPI(w http.ResponseWriter, r *http.Request) {
	requestID, err := generateRequestID()
	if err != nil {
		JSONError(w, "Internal Server Error", http.StatusInternalServerError)
		ws.logger.Error("Failed to generate request ID", "err", err)
		return
	}

	request, err := service_registry.ConvertHttpRequestToConsensusRequest(r, requestID)
	if err != nil {
		JSONError(w, "Failed to convert request: "+err.Error(), http.StatusUnprocessableEntity)
		ws.logger.Error("Failed to convert HTTP request", "err", err)
		return
	}
	// request.Body = strings.TrimSpace(request.Body)

	response, err := request.GenerateResponse(ws.serviceRegistry)
	if err != nil {
		JSONError(w, "Failed to generate response: "+err.Error(), http.StatusUnprocessableEntity)
		ws.logger.Error("Failed to generate response", "err", err)
		return
	}

	transaction := &service_registry.Transaction{
		Request:      *request,
		Response:     *response,
		OriginNodeID: string(ws.node.ConsensusReactor().Switch.NodeInfo().ID()),
	}

	// Respond to client
	apiResponse := ClientResponse{
		StatusCode: response.StatusCode,
		Headers:    response.Headers,
		Body:       response.ParseBody(),
		// Meta: TransactionStatus{
		// 	TxID:        consensusResponse.TxHash,
		// 	RequestID:   requestID,
		// 	Status:      "confirmed",
		// 	BlockHeight: blockHeight,
		// 	// BlockHash:   hex.EncodeToString(consensusResponse.Hash),
		// 	ConfirmTime: time.Now(),
		// 	ResponseInfo: ResponseInfo{
		// 		StatusCode:  response.StatusCode,
		// 		ContentType: response.Headers["Content-Type"],
		// 		BodyLength:  len(response.Body),
		// 	},
		// 	BlockTxs: BlockTxsDetail{
		// 		BlockTransactions:    blockTransactions,
		// 		BlockTransactionsB64: blockTransactionsB64,
		// 	},
		// },
		NodeID: transaction.OriginNodeID,
	}

	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(apiResponse)
	if err != nil {
		ws.logger.Error("Failed to encode client response", "err", err)
	}

	ws.logger.Info("=== Req-Res Pair Result ===",
		transaction.Request.Path,
		transaction.Request.Method,
		transaction.Request.Body,
		transaction.Response.StatusCode,
		transaction.Response.Body,
	)
}

// checkTransactionStatus checks the status of a transaction in the blockchain
func (ws *WebServer) checkTransactionStatus(txID string) (*TransactionStatus, error) {
	// Query the blockchain for the transaction
	ws.logger.Info("WEBSERVER CHECK TRANSACTION STATUS 1")
	query := fmt.Sprintf("tx.hash='%s'", txID)
	res, err := ws.cometBftRpcClient.TxSearch(context.Background(), query, false, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("error searching for transaction: %w", err)
	}

	consensusInfo := ConsensusInfo{
		AgreementNodes: 0,               // We'll calculate this
		NodeResponses:  make([]bool, 0), // Track individual node responses
	}

	if len(res.Txs) == 0 {
		return nil, nil // Transaction not found
	}

	tx := res.Txs[0]

	// Parse the transaction
	var completeTx service_registry.Transaction
	err = json.Unmarshal(tx.Tx, &completeTx)
	if err != nil {
		return nil, fmt.Errorf("error parsing transaction: %w", err)
	}

	// Extract events
	status := "pending"
	for _, event := range tx.TxResult.Events {
		if event.Type == "tm.event.Vote" || event.Type == "vote" {
			consensusInfo.AgreementNodes++
			consensusInfo.NodeResponses = append(consensusInfo.NodeResponses, true)
		}
		if event.Type == "dews_tx" {
			for _, attr := range event.Attributes {
				if string(attr.Key) == "status" {
					status = string(attr.Value)
				}
			}
		}
	}

	block, err := ws.cometBftRpcClient.Block(context.Background(), &tx.Height)
	if err != nil {
		return nil, fmt.Errorf("error getting block: %w", err)
	}
	if block.Block == nil {
		ws.logger.Info("Web Server", "Block not found")
	}
	ws.logger.Info("Web Server", "Block", block)

	// Create response info
	responseInfo := ResponseInfo{
		StatusCode:  completeTx.Response.StatusCode,
		ContentType: completeTx.Response.Headers["Content-Type"],
		BodyLength:  len(completeTx.Response.Body),
	}

	var blockTransactionsB64 []string
	var blockTransactions []service_registry.Transaction
	for _, tx := range block.Block.Txs {
		// Convert the transaction bytes to base64
		b64Tx := base64.StdEncoding.EncodeToString(tx)
		blockTransactionsB64 = append(blockTransactionsB64, b64Tx)

		// Try to parse the transaction
		var parsedTx service_registry.Transaction
		if err := json.Unmarshal(tx, &parsedTx); err == nil {
			parsedTx.Response.BodyInterface = parsedTx.Response.ParseBody()
			blockTransactions = append(blockTransactions, parsedTx)
		} else {
			// If parsing fails, you might want to log the error
			ws.logger.Error("Failed to parse transaction", "err", err)
		}
	}

	// Create transaction status
	txStatus := &TransactionStatus{
		TxID:         txID,
		RequestID:    completeTx.Request.RequestID,
		Status:       status,
		BlockHeight:  tx.Height,
		BlockHash:    fmt.Sprintf("%X", tx.Hash),
		ConfirmTime:  time.Unix(0, time.Now().Unix()), // TODO
		ResponseInfo: responseInfo,
		BlockTxs: BlockTxsDetail{
			BlockTransactions:    blockTransactions,
			BlockTransactionsB64: blockTransactionsB64,
		},
	}

	return txStatus, nil
}

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

// JSONError sends a JSON formatted error response with the given status code and message
func JSONError(w http.ResponseWriter, message string, statusCode int) {
	errorResponse := struct {
		Error string `json:"error"`
	}{
		Error: message,
	}
	jsonBytes, err := json.Marshal(errorResponse)
	if err != nil {
		// If JSON marshaling fails, fall back to plain text
		JSONError(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set content type and status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Write JSON response
	w.Write(jsonBytes)
}
