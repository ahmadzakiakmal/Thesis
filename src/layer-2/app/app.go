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

	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository"
	"github.com/ahmadzakiakmal/thesis/src/layer-2/srvreg"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/dgraph-io/badger/v4"
)

// Application implements the ABCI interface for the nodes
type Application struct {
	badgerDB        *badger.DB
	onGoingBlock    *badger.Txn
	serviceRegistry *srvreg.ServiceRegistry
	nodeID          string
	mu              sync.Mutex
	config          *AppConfig
	logger          cmtlog.Logger
	repository      *repository.Repository
}

// AppConfig contains configuration for the application
type AppConfig struct {
	NodeID        string
	RequiredVotes int  // Number of votes required for consensus
	LogAllTxs     bool // Whether to log all transactions, even failed ones
}

// NewABCIApplication creates a new  application
func NewABCIApplication(badgerDB *badger.DB, serviceRegistry *srvreg.ServiceRegistry, config *AppConfig, logger cmtlog.Logger, repository *repository.Repository) *Application {
	return &Application{
		badgerDB:        badgerDB,
		serviceRegistry: serviceRegistry,
		nodeID:          "",
		config:          config,
		logger:          logger,
		repository:      repository,
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
	txBytes := check.Tx

	var tx srvreg.Transaction
	err := json.Unmarshal(txBytes, &tx)
	if err != nil {
		return &abcitypes.CheckTxResponse{
				Code: 1,
			},
			fmt.Errorf(
				"fail to parse tx on CheckTx: %s",
				err.Error(),
			)
	}

	return &abcitypes.CheckTxResponse{
		Code: 0,
	}, nil
}

// InitChain implements the ABCI InitChain method
func (app *Application) InitChain(_ context.Context, chain *abcitypes.InitChainRequest) (*abcitypes.InitChainResponse, error) {
	// Initialize the application state
	return &abcitypes.InitChainResponse{}, nil
}

// PrepareProposal implements the ABCI PrepareProposal method
func (app *Application) PrepareProposal(_ context.Context, proposal *abcitypes.PrepareProposalRequest) (*abcitypes.PrepareProposalResponse, error) {
	// Include all transactions
	return &abcitypes.PrepareProposalResponse{Txs: proposal.Txs}, nil
}

// ProcessProposal implements the ABCI ProcessProposal method
func (app *Application) ProcessProposal(
	_ context.Context,
	proposal *abcitypes.ProcessProposalRequest,
) (*abcitypes.ProcessProposalResponse, error) {
	fmt.Println("[PROCESSPROPOSAL]:")
	for _, txBytes := range proposal.Txs {
		var tx *srvreg.Transaction
		json.Unmarshal(txBytes, &tx)

		isTxOriginator := app.nodeID == tx.OriginNodeID
		if !isTxOriginator {
			// Replicate the request and compare the response
			handler, isHandlerFound := app.serviceRegistry.
				GetHandlerForPath(
					tx.Request.Method,
					tx.Request.Path,
				)
			if isHandlerFound {
				response, err := handler(&tx.Request)
				if err != nil {
					fmt.Println("Voted invalid", err)
					return &abcitypes.ProcessProposalResponse{
						Status: abcitypes.PROCESS_PROPOSAL_STATUS_REJECT,
					}, err
				}
				if !compareResponses(response, &tx.Response) {
					fmt.Println("Voted invalid")
					fmt.Println("Different responses, byzantine behavior detected")
					return &abcitypes.ProcessProposalResponse{
							Status: abcitypes.
								PROCESS_PROPOSAL_STATUS_REJECT,
						},
						fmt.Errorf("response is different, byzantine behavior detected")
				}
			} else {
				fmt.Println("Voted invalid", "Handler not found")
				return &abcitypes.ProcessProposalResponse{
						Status: abcitypes.PROCESS_PROPOSAL_STATUS_REJECT,
					},
					fmt.Errorf("handler not found, byzantine behavior detected")
			}
		}
	}
	fmt.Println("Voted valid")
	return &abcitypes.ProcessProposalResponse{Status: abcitypes.
		PROCESS_PROPOSAL_STATUS_ACCEPT,
	}, nil
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
		var tx srvreg.Transaction

		if err := json.Unmarshal(txBytes, &tx); err != nil {
			txResults[i] = &abcitypes.ExecTxResult{
				Code: 1,
				Log:  "Invalid transaction format",
			}
			continue
		}

		txID := generateTxID(
			tx.Request.RequestID,
			tx.OriginNodeID,
		)
		// accept all tx that made it through to this method
		status := "accepted"
		txResults[i] = app.storeTransaction(
			txID,
			&tx,
			status,
			txBytes,
		)
	}

	// store the last block info
	blockHeight := req.Height

	// calculate application hash
	appHash := calculateAppHash(txResults)

	// store block info
	err := app.onGoingBlock.
		Set(
			[]byte("last_block_height"),
			int64ToBytes(blockHeight),
		)
	if err != nil {
		log.Printf(
			"Error storing block height: %v",
			err,
		)
	}

	err = app.onGoingBlock.
		Set(
			[]byte("last_block_app_hash"),
			appHash,
		)
	if err != nil {
		log.Printf(
			"Error storing app hash: %v",
			err,
		)
	}

	return &abcitypes.FinalizeBlockResponse{
		TxResults: txResults,
		AppHash:   appHash,
	}, err
}

// Commit implements the ABCI Commit method
func (app *Application) Commit(_ context.Context, commit *abcitypes.CommitRequest) (*abcitypes.CommitResponse, error) {
	// Commit changes to the database
	err := app.onGoingBlock.Commit()
	if err != nil {
		log.Printf("Error committing block: %v", err)
	}

	return &abcitypes.CommitResponse{}, nil
}

// ListSnapshots implements the ABCI ListSnapshots method
func (app *Application) ListSnapshots(_ context.Context, snapshots *abcitypes.ListSnapshotsRequest) (*abcitypes.ListSnapshotsResponse, error) {
	return &abcitypes.ListSnapshotsResponse{}, nil
}

// OfferSnapshot implements the ABCI OfferSnapshot method
func (app *Application) OfferSnapshot(_ context.Context, snapshot *abcitypes.OfferSnapshotRequest) (*abcitypes.OfferSnapshotResponse, error) {
	return &abcitypes.OfferSnapshotResponse{}, nil
}

// LoadSnapshotChunk implements the ABCI LoadSnapshotChunk method
func (app *Application) LoadSnapshotChunk(_ context.Context, chunk *abcitypes.LoadSnapshotChunkRequest) (*abcitypes.LoadSnapshotChunkResponse, error) {
	return &abcitypes.LoadSnapshotChunkResponse{}, nil
}

// ApplySnapshotChunk implements the ABCI ApplySnapshotChunk method
func (app *Application) ApplySnapshotChunk(_ context.Context, chunk *abcitypes.ApplySnapshotChunkRequest) (*abcitypes.ApplySnapshotChunkResponse, error) {
	return &abcitypes.ApplySnapshotChunkResponse{
		Result: abcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_ACCEPT,
	}, nil
}

// ExtendVote implements the ABCI ExtendVote method
func (app *Application) ExtendVote(_ context.Context, extend *abcitypes.ExtendVoteRequest) (*abcitypes.ExtendVoteResponse, error) {
	return &abcitypes.ExtendVoteResponse{}, nil
}

// VerifyVoteExtension implements the ABCI VerifyVoteExtension method
func (app *Application) VerifyVoteExtension(_ context.Context, verify *abcitypes.VerifyVoteExtensionRequest) (*abcitypes.VerifyVoteExtensionResponse, error) {
	return &abcitypes.VerifyVoteExtensionResponse{}, nil
}

// Helper Functions

// storeTransaction stores the transaction in the database
func (app *Application) storeTransaction(txID string, tx *srvreg.Transaction, status string, rawTx []byte) *abcitypes.ExecTxResult {
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
func compareResponses(a, b *srvreg.Response) bool {
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
