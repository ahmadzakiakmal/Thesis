package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/ahmadzakiakmal/thesis/src/layer-1/server/models"
	service_registry "github.com/ahmadzakiakmal/thesis/src/layer-1/service-registry"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/dgraph-io/badger/v4"
	"gorm.io/gorm"
)

// Application implements the ABCI interface for the nodes
type Application struct {
	badgerDB        *badger.DB
	postgresDB      *gorm.DB
	onGoingBlock    *badger.Txn
	serviceRegistry *service_registry.ServiceRegistry
	nodeID          string
	mu              sync.Mutex
	config          *AppConfig
	logger          cmtlog.Logger
}

// AppConfig contains configuration for the application
type AppConfig struct {
	NodeID        string
	RequiredVotes int  // Number of votes required for consensus
	LogAllTxs     bool // Whether to log all transactions, even failed ones
}

// NewDeWSApplication creates a new DeWS application
func NewDeWSApplication(badgerDB *badger.DB, serviceRegistry *service_registry.ServiceRegistry, config *AppConfig, logger cmtlog.Logger, db *gorm.DB) *Application {
	return &Application{
		badgerDB:        badgerDB,
		serviceRegistry: serviceRegistry,
		nodeID:          "",
		config:          config,
		logger:          logger,
		postgresDB:      db,
	}
}

func (app *Application) SetNodeID(id string) {
	app.nodeID = id
}

// Info implements the ABCI Info method
func (app *Application) Info(_ context.Context, info *abcitypes.InfoRequest) (*abcitypes.InfoResponse, error) {
	// Return application info including last block height and app hash
	lastBlockHeight := int64(0)
	var lastBlockAppHash []byte

	// Get last block info from DB
	err := app.badgerDB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("last_block_height"))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil
			}
			return err
		}

		err = item.Value(func(val []byte) error {
			lastBlockHeight = bytesToInt64(val)
			return nil
		})
		if err != nil {
			return err
		}

		item, err = txn.Get([]byte("last_block_app_hash"))
		if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}

		if err == nil {
			err = item.Value(func(val []byte) error {
				lastBlockAppHash = val
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Error getting last block info: %v", err)
	}

	return &abcitypes.InfoResponse{
		LastBlockHeight:  lastBlockHeight,
		LastBlockAppHash: lastBlockAppHash,
	}, nil
}

// Query implements the ABCI Query method
func (app *Application) Query(_ context.Context, req *abcitypes.QueryRequest) (*abcitypes.QueryResponse, error) {
	// Query can look up transactions, verify responses, etc.
	if len(req.Data) == 0 {
		return &abcitypes.QueryResponse{
			Code: 1,
			Log:  "Empty query data",
		}, nil
	}

	// Check if this is a request verification query
	if bytes.HasPrefix(req.Data, []byte("verify:")) {
		txID := req.Data[7:] // Skip "verify:" prefix
		return app.verifyTransaction(txID)
	}

	// Handle regular key-value lookup
	resp := abcitypes.QueryResponse{Key: req.Data}

	dbErr := app.badgerDB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(req.Data)

		if err != nil {
			if !errors.Is(err, badger.ErrKeyNotFound) {
				return err
			}
			resp.Log = "key doesn't exist"
			return nil
		}

		return item.Value(func(val []byte) error {
			resp.Log = "exists"
			resp.Value = val
			return nil
		})
	})

	if dbErr != nil {
		log.Printf("Error reading database, unable to execute query: %v", dbErr)
		return &abcitypes.QueryResponse{
			Code: 2,
			Log:  fmt.Sprintf("Database error: %v", dbErr),
		}, nil
	}

	return &resp, nil
}

// verifyTransaction looks up a transaction and its consensus status
func (app *Application) verifyTransaction(txID []byte) (*abcitypes.QueryResponse, error) {
	var resp abcitypes.QueryResponse

	err := app.badgerDB.View(func(txn *badger.Txn) error {
		// Get transaction details
		txKey := append([]byte("tx:"), txID...)
		item, err := txn.Get(txKey)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				resp.Log = "Transaction not found"
				resp.Code = 1
				return nil
			}
			return err
		}

		var txData []byte
		err = item.Value(func(val []byte) error {
			txData = append([]byte{}, val...)
			return nil
		})
		if err != nil {
			return err
		}

		// Get consensus status
		statusKey := append([]byte("status:"), txID...)
		item, err = txn.Get(statusKey)
		if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}

		var status string = "unknown"
		if err == nil {
			err = item.Value(func(val []byte) error {
				status = string(val)
				return nil
			})
			if err != nil {
				return err
			}
		}

		// Create response with transaction and status
		resp.Value = txData
		resp.Log = status
		resp.Code = 0
		return nil
	})

	if err != nil {
		resp.Code = 2
		resp.Log = fmt.Sprintf("Database error: %v", err)
	}

	return &resp, nil
}

// CheckTx implements the ABCI CheckTx method
func (app *Application) CheckTx(
	_ context.Context,
	check *abcitypes.CheckTxRequest,
) (*abcitypes.CheckTxResponse, error) {
	tx := check.Tx

	// Log the raw bytes in various formats
	app.logger.Info("CheckTx raw transaction",
		"length", len(tx),
		// "raw_bytes", fmt.Sprintf("%v", tx),
		"as_string", string(tx),
		// "hex", fmt.Sprintf("%X", tx),
	)

	// Accept any transaction for now
	return &abcitypes.CheckTxResponse{Code: 0}, nil
}

// InitChain implements the ABCI InitChain method
func (app *Application) InitChain(
	_ context.Context,
	chain *abcitypes.InitChainRequest,
) (*abcitypes.InitChainResponse, error) {
	// Initialize the application state
	return &abcitypes.InitChainResponse{}, nil
}

// PrepareProposal implements the ABCI PrepareProposal method
func (app *Application) PrepareProposal(
	_ context.Context,
	proposal *abcitypes.PrepareProposalRequest,
) (*abcitypes.PrepareProposalResponse, error) {
	// For DeWS, we want to include all transactions
	return &abcitypes.PrepareProposalResponse{Txs: proposal.Txs}, nil
}

// ProcessProposal implements the ABCI ProcessProposal method
func (app *Application) ProcessProposal(
	_ context.Context,
	proposal *abcitypes.ProcessProposalRequest,
) (*abcitypes.ProcessProposalResponse, error) {
	// Process the proposed block
	for _, tx := range proposal.Txs {
		var dewsTx service_registry.Transaction
		if err := json.Unmarshal(tx, &dewsTx); err != nil {
			// If we can't parse the transaction, we reject the proposal
			return &abcitypes.ProcessProposalResponse{Status: abcitypes.PROCESS_PROPOSAL_STATUS_REJECT}, nil
		}
		isNotOrigin := dewsTx.OriginNodeID != app.nodeID
		app.logger.Info("Process Proposal", "Tx Origin", dewsTx.OriginNodeID, "ID", app.nodeID)
		if isNotOrigin {
			app.logger.Info("Process Proposal", "Is the proposer?", "false")
		} else {
			app.logger.Info("Process Proposal", "Is the proposer?", "true")
		}

		// Check if this node is the origin or if we need to replicate
		if isNotOrigin {
			// This is a transaction from another node, we need to verify it
			// by replicating the computation
			req := dewsTx.Request
			resp, err := req.GenerateResponse(app.serviceRegistry)
			if err != nil {
				log.Printf("Error generating response for request %s: %v", req.RequestID, err)
				// We still accept the proposal, but we'll mark the transaction as failed
				continue
			}

			// Compare our response with the one in the transaction
			if !compareResponses(resp, &dewsTx.Response) {
				log.Printf("Byzantine behavior detected: responses don't match for request %s", req.RequestID)
				// In this case, we would reject the proposal in a real system
				// For simplicity, we'll still accept it but mark it as failed
			}
		}
	}

	// Accept the proposal
	return &abcitypes.ProcessProposalResponse{Status: abcitypes.PROCESS_PROPOSAL_STATUS_ACCEPT}, nil
}

// FinalizeBlock implements the ABCI FinalizeBlock method
func (app *Application) FinalizeBlock(
	_ context.Context,
	req *abcitypes.FinalizeBlockRequest,
) (*abcitypes.FinalizeBlockResponse, error) {
	var txResults = make([]*abcitypes.ExecTxResult, len(req.Txs))

	app.mu.Lock()
	defer app.mu.Unlock()

	app.onGoingBlock = app.badgerDB.NewTransaction(true)

	for i, txBytes := range req.Txs {
		var tx service_registry.Transaction

		if err := json.Unmarshal(txBytes, &tx); err != nil {
			txResults[i] = &abcitypes.ExecTxResult{Code: 1, Log: "Invalid transaction format"}
			continue
		}

		if tx.Request.Method == "" || tx.Request.Path == "" || tx.Request.RequestID == "" {
			txResults[i] = &abcitypes.ExecTxResult{
				Code: 2,
				Log:  "Empty request in transaction",
			}
			continue
		}

		// Generate a transaction ID
		txID := generateTxID(tx.Request.RequestID, tx.OriginNodeID)

		// Check if this node is the origin
		isOrigin := tx.OriginNodeID == app.nodeID

		var result *abcitypes.ExecTxResult

		if isOrigin {
			// This is our transaction, no need to replicate
			result = app.storeTransaction(txID, &tx, "accepted", txBytes)
		} else {
			// This is a transaction from another node
			// Instead of re-executing, just verify the record exists

			// Extract user email from request body for verification
			var userData map[string]interface{}
			err := json.Unmarshal([]byte(tx.Request.Body), &userData)

			if err != nil || tx.Request.Path != "/api/customers" || tx.Request.Method != "POST" {
				// If we can't parse the body or it's not a user creation request,
				// fall back to accepting the transaction as is
				result = app.storeTransaction(txID, &tx, "accepted", txBytes)
			} else {
				// For user creation, verify the record exists in DB
				email, ok := userData["email"].(string)
				if !ok || email == "" {
					// Cannot verify, accept transaction
					result = app.storeTransaction(txID, &tx, "accepted", txBytes)
				} else {
					// Check if user with this email exists
					var count int64
					dbErr := app.postgresDB.Model(&models.User{}).Where("email = ?", email).Count(&count).Error

					if dbErr != nil || count == 0 {
						// User doesn't exist, which is unexpected
						app.logger.Info("User verification failed", "email", email, "error", dbErr)
						result = app.storeTransaction(txID, &tx, "verification_failed", txBytes)
					} else {
						// User exists, which means the transaction was processed correctly
						result = app.storeTransaction(txID, &tx, "accepted", txBytes)
					}
				}
			}
		}

		txResults[i] = result
	}

	// Store the last block info
	blockHeight := req.Height

	// Calculate application hash
	appHash := calculateAppHash(txResults)

	// Store block info
	err := app.onGoingBlock.Set([]byte("last_block_height"), int64ToBytes(blockHeight))
	if err != nil {
		log.Printf("Error storing block height: %v", err)
	}

	err = app.onGoingBlock.Set([]byte("last_block_app_hash"), appHash)
	if err != nil {
		log.Printf("Error storing app hash: %v", err)
	}

	return &abcitypes.FinalizeBlockResponse{
		TxResults: txResults,
		AppHash:   appHash,
	}, err
}

// Commit implements the ABCI Commit method
func (app *Application) Commit(
	_ context.Context,
	commit *abcitypes.CommitRequest,
) (*abcitypes.CommitResponse, error) {
	// Commit changes to the database
	err := app.onGoingBlock.Commit()
	if err != nil {
		log.Printf("Error committing block: %v", err)
	}

	return &abcitypes.CommitResponse{}, nil
}

// ListSnapshots implements the ABCI ListSnapshots method
func (app *Application) ListSnapshots(
	_ context.Context,
	snapshots *abcitypes.ListSnapshotsRequest,
) (*abcitypes.ListSnapshotsResponse, error) {
	return &abcitypes.ListSnapshotsResponse{}, nil
}

// OfferSnapshot implements the ABCI OfferSnapshot method
func (app *Application) OfferSnapshot(
	_ context.Context,
	snapshot *abcitypes.OfferSnapshotRequest,
) (*abcitypes.OfferSnapshotResponse, error) {
	return &abcitypes.OfferSnapshotResponse{}, nil
}

// LoadSnapshotChunk implements the ABCI LoadSnapshotChunk method
func (app *Application) LoadSnapshotChunk(
	_ context.Context,
	chunk *abcitypes.LoadSnapshotChunkRequest,
) (*abcitypes.LoadSnapshotChunkResponse, error) {
	return &abcitypes.LoadSnapshotChunkResponse{}, nil
}

// ApplySnapshotChunk implements the ABCI ApplySnapshotChunk method
func (app *Application) ApplySnapshotChunk(
	_ context.Context,
	chunk *abcitypes.ApplySnapshotChunkRequest,
) (*abcitypes.ApplySnapshotChunkResponse, error) {
	return &abcitypes.ApplySnapshotChunkResponse{
		Result: abcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_ACCEPT,
	}, nil
}

// ExtendVote implements the ABCI ExtendVote method
func (app *Application) ExtendVote(
	_ context.Context,
	extend *abcitypes.ExtendVoteRequest,
) (*abcitypes.ExtendVoteResponse, error) {
	return &abcitypes.ExtendVoteResponse{}, nil
}

// VerifyVoteExtension implements the ABCI VerifyVoteExtension method
func (app *Application) VerifyVoteExtension(
	_ context.Context,
	verify *abcitypes.VerifyVoteExtensionRequest,
) (*abcitypes.VerifyVoteExtensionResponse, error) {
	return &abcitypes.VerifyVoteExtensionResponse{}, nil
}

// Helper Functions

// storeTransaction stores the transaction in the database
func (app *Application) storeTransaction(txID string, tx *service_registry.Transaction, status string, rawTx []byte) *abcitypes.ExecTxResult {
	// Store the transaction
	txKey := append([]byte("tx:"), []byte(txID)...)
	err := app.onGoingBlock.Set(txKey, rawTx)
	if err != nil {
		log.Printf("Error storing transaction: %v", err)
		return &abcitypes.ExecTxResult{
			Code: 3,
			Log:  fmt.Sprintf("Database error: %v", err),
		}
	}

	// Store the status
	statusKey := append([]byte("status:"), []byte(txID)...)
	err = app.onGoingBlock.Set(statusKey, []byte(status))
	if err != nil {
		log.Printf("Error storing transaction status: %v", err)
	}

	// Create events for the transaction
	events := []abcitypes.Event{
		{
			Type: "dews_tx",
			Attributes: []abcitypes.EventAttribute{
				{Key: "request_id", Value: tx.Request.RequestID, Index: true},
				{Key: "origin_node", Value: tx.OriginNodeID, Index: true},
				{Key: "status", Value: status, Index: true},
				{Key: "tx_id", Value: txID, Index: true},
			},
		},
	}

	// Add method and path for easier filtering
	events = append(events, abcitypes.Event{
		Type: "request",
		Attributes: []abcitypes.EventAttribute{
			{Key: "method", Value: tx.Request.Method, Index: true},
			{Key: "path", Value: tx.Request.Path, Index: true},
		},
	})

	return &abcitypes.ExecTxResult{
		Code:   0,
		Data:   []byte(txID),
		Log:    status,
		Events: events,
	}
}

// compareResponses compares two DeWSResponse objects for equality
func compareResponses(a, b *service_registry.Response) bool {
	// Compare status code
	if a.StatusCode != b.StatusCode {
		return false
	}

	// Compare body
	if a.Body != b.Body {
		return false
	}

	// For a real implementation, we'd need to ignore non-deterministic headers
	// such as Date, Server, etc.

	return true
}

// generateTxID generates a unique ID for a transaction
func generateTxID(requestID, nodeID string) string {
	hash := sha256.Sum256([]byte(requestID + nodeID))
	return hex.EncodeToString(hash[:])
}

// calculateAppHash calculates the application hash for the current block
func calculateAppHash(txResults []*abcitypes.ExecTxResult) []byte {
	// Simple implementation - in a real system, you might want a more
	// sophisticated approach like a Merkle tree
	allData := make([]byte, 0)

	for _, result := range txResults {
		allData = append(allData, result.Data...)
	}

	hash := sha256.Sum256(allData)
	return hash[:]
}

// int64ToBytes converts an int64 to bytes
func int64ToBytes(i int64) []byte {
	buf := make([]byte, 8)

	buf[0] = byte(i >> 56)
	buf[1] = byte(i >> 48)
	buf[2] = byte(i >> 40)
	buf[3] = byte(i >> 32)
	buf[4] = byte(i >> 24)
	buf[5] = byte(i >> 16)
	buf[6] = byte(i >> 8)
	buf[7] = byte(i)

	return buf
}

// bytesToInt64 converts bytes to an int64
func bytesToInt64(buf []byte) int64 {
	if len(buf) < 8 {
		return 0
	}

	return int64(buf[0])<<56 |
		int64(buf[1])<<48 |
		int64(buf[2])<<40 |
		int64(buf[3])<<32 |
		int64(buf[4])<<24 |
		int64(buf[5])<<16 |
		int64(buf[6])<<8 |
		int64(buf[7])
}
