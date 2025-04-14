package sequencer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	service_registry "github.com/ahmadzakiakmal/thesis/src/layer-2-comet/service-registry"
	cmtlog "github.com/cometbft/cometbft/libs/log"
)

// SequencerConfig contains configuration for the sequencer
type SequencerConfig struct {
	NodeID            string
	L1Endpoint        string
	BatchSizeLimit    int           // Maximum number of transactions in a batch
	BatchTimeLimit    time.Duration // Maximum time to wait before creating a batch
	L1CommitInterval  time.Duration // How often to commit to L1
	EnableL1Commits   bool          // Whether to commit state to L1
	SequencerMode     bool          // Whether this node is a sequencer
	DisableBatching   bool          // For debugging: don't batch, process immediately
	ParallelExecution bool          // Whether to execute transactions in parallel
}

// DefaultSequencerConfig returns a default sequencer configuration
func DefaultSequencerConfig() *SequencerConfig {
	return &SequencerConfig{
		NodeID:            "l2-sequencer",
		L1Endpoint:        "http://localhost:5000",
		BatchSizeLimit:    100,
		BatchTimeLimit:    2 * time.Second,
		L1CommitInterval:  60 * time.Second,
		EnableL1Commits:   true,
		SequencerMode:     true,
		DisableBatching:   false,
		ParallelExecution: true,
	}
}

// SequencerBatch represents a batch of transactions
type SequencerBatch struct {
	BatchID        string                             `json:"batch_id"`
	Transactions   []service_registry.DeWSTransaction `json:"transactions"`
	StateRoot      []byte                             `json:"state_root"`
	PrevStateRoot  []byte                             `json:"prev_state_root"`
	Timestamp      time.Time                          `json:"timestamp"`
	SequencerID    string                             `json:"sequencer_id"`
	TransactionIDs []string                           `json:"transaction_ids"`
	BatchNumber    uint64                             `json:"batch_number"`
	L1Committed    bool                               `json:"l1_committed"`
	L1TxHash       string                             `json:"l1_tx_hash,omitempty"`
}

// TransactionStatus represents the status of a transaction
type TransactionStatus struct {
	TxID         string    `json:"tx_id"`
	Status       string    `json:"status"` // "pending", "included", "committed", "finalized"
	BatchID      string    `json:"batch_id,omitempty"`
	IncludedTime time.Time `json:"included_time,omitempty"`
	CommitTime   time.Time `json:"commit_time,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// Sequencer manages transaction batching and L1 commitments
type Sequencer struct {
	config          *SequencerConfig
	logger          cmtlog.Logger
	pendingTxs      [][]byte
	pendingTxsLock  sync.Mutex
	batches         map[string]*SequencerBatch
	batchesLock     sync.RWMutex
	txStatus        map[string]*TransactionStatus
	txStatusLock    sync.RWMutex
	batchNumber     uint64
	lastStateRoot   []byte
	l1Client        *http.Client
	stopCh          chan struct{}
	serviceRegistry *service_registry.ServiceRegistry
}

// NewSequencer creates a new sequencer
func NewSequencer(config *SequencerConfig, logger cmtlog.Logger, serviceRegistry *service_registry.ServiceRegistry) *Sequencer {
	// Initialize with reasonable defaults if not provided
	if logger == nil {
		logger = cmtlog.NewNopLogger()
	}

	return &Sequencer{
		config:          config,
		logger:          logger,
		pendingTxs:      make([][]byte, 0, config.BatchSizeLimit),
		batches:         make(map[string]*SequencerBatch),
		txStatus:        make(map[string]*TransactionStatus),
		batchNumber:     0,
		lastStateRoot:   make([]byte, 32), // Initial empty state root
		l1Client:        &http.Client{Timeout: 30 * time.Second},
		stopCh:          make(chan struct{}),
		serviceRegistry: serviceRegistry,
	}
}

// Start begins the sequencer operations
func (s *Sequencer) Start(ctx context.Context) error {
	s.logger.Info("Starting sequencer", "node_id", s.config.NodeID)

	// Start the batch processor
	if !s.config.DisableBatching {
		go s.batchProcessor(ctx)
	}

	// Start L1 commitment service if enabled
	if s.config.EnableL1Commits {
		go s.l1CommitmentService(ctx)
	}

	s.logger.Info("Sequencer started successfully")
	return nil
}

// GetConfig returns the sequencer configuration
func (s *Sequencer) GetConfig() *SequencerConfig {
	return s.config
}

// Stop halts the sequencer operations
func (s *Sequencer) Stop() {
	s.logger.Info("Stopping sequencer")
	close(s.stopCh)
}

// SubmitTransaction submits a transaction to the sequencer
func (s *Sequencer) SubmitTransaction(txBytes []byte) (string, error) {
	// Generate a transaction ID
	txID := generateTxID(txBytes)

	// If batching is disabled, process immediately
	if s.config.DisableBatching {
		return s.processTransactionDirectly(txBytes, txID)
	}

	// Otherwise, add to pending batch
	s.pendingTxsLock.Lock()
	defer s.pendingTxsLock.Unlock()

	// Register the transaction status
	s.txStatusLock.Lock()
	s.txStatus[txID] = &TransactionStatus{
		TxID:   txID,
		Status: "pending",
	}
	s.txStatusLock.Unlock()

	// Add to pending transactions
	s.pendingTxs = append(s.pendingTxs, txBytes)
	s.logger.Info("Transaction submitted to batch", "tx_id", txID, "pending_count", len(s.pendingTxs))

	// If we've reached the batch size limit, trigger processing
	if len(s.pendingTxs) >= s.config.BatchSizeLimit {
		go s.triggerBatchProcessing()
	}

	return txID, nil
}

// GetTransactionStatus returns the status of a transaction
func (s *Sequencer) GetTransactionStatus(txID string) (*TransactionStatus, error) {
	s.txStatusLock.RLock()
	defer s.txStatusLock.RUnlock()

	status, exists := s.txStatus[txID]
	if !exists {
		return nil, fmt.Errorf("transaction %s not found", txID)
	}

	return status, nil
}

// GetBatch returns a batch by ID
func (s *Sequencer) GetBatch(batchID string) (*SequencerBatch, error) {
	s.batchesLock.RLock()
	defer s.batchesLock.RUnlock()

	batch, exists := s.batches[batchID]
	if !exists {
		return nil, fmt.Errorf("batch %s not found", batchID)
	}

	return batch, nil
}

// Internal functions

// processTransactionDirectly processes a transaction without batching
func (s *Sequencer) processTransactionDirectly(txBytes []byte, txID string) (string, error) {
	var tx service_registry.DeWSTransaction
	if err := json.Unmarshal(txBytes, &tx); err != nil {
		return "", fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	// Execute the transaction
	resp, err := s.executeTransaction(&tx)
	if err != nil {
		return "", fmt.Errorf("failed to execute transaction: %w", err)
	}

	// Update transaction with response
	tx.Response = *resp

	// Register as a single-tx batch for consistency
	batchID := generateBatchID([][]byte{txBytes})
	batch := &SequencerBatch{
		BatchID:        batchID,
		Transactions:   []service_registry.DeWSTransaction{tx},
		StateRoot:      calculateStateRoot([]service_registry.DeWSTransaction{tx}),
		PrevStateRoot:  s.lastStateRoot,
		Timestamp:      time.Now(),
		SequencerID:    s.config.NodeID,
		TransactionIDs: []string{txID},
		BatchNumber:    s.batchNumber,
	}

	s.batchNumber++
	s.lastStateRoot = batch.StateRoot

	// Store the batch
	s.batchesLock.Lock()
	s.batches[batchID] = batch
	s.batchesLock.Unlock()

	// Update transaction status
	s.txStatusLock.Lock()
	s.txStatus[txID] = &TransactionStatus{
		TxID:         txID,
		Status:       "included",
		BatchID:      batchID,
		IncludedTime: time.Now(),
	}
	s.txStatusLock.Unlock()

	return txID, nil
}

// triggerBatchProcessing manually triggers batch processing
func (s *Sequencer) triggerBatchProcessing() {
	s.pendingTxsLock.Lock()
	if len(s.pendingTxs) == 0 {
		s.pendingTxsLock.Unlock()
		return
	}

	// Take the current pending transactions
	txs := make([][]byte, len(s.pendingTxs))
	copy(txs, s.pendingTxs)
	s.pendingTxs = make([][]byte, 0, s.config.BatchSizeLimit)
	s.pendingTxsLock.Unlock()

	// Process them
	s.processBatch(txs)
}

// batchProcessor is the background routine that processes batches
func (s *Sequencer) batchProcessor(ctx context.Context) {
	ticker := time.NewTicker(s.config.BatchTimeLimit)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.triggerBatchProcessing()
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		}
	}
}

// l1CommitmentService periodically commits state to L1
func (s *Sequencer) l1CommitmentService(ctx context.Context) {
	ticker := time.NewTicker(s.config.L1CommitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.commitStateToL1(); err != nil {
				s.logger.Error("Failed to commit state to L1", "error", err)
			}
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		}
	}
}

// processBatch processes a batch of transactions
func (s *Sequencer) processBatch(txs [][]byte) {
	if len(txs) == 0 {
		return
	}

	s.logger.Info("Processing batch", "tx_count", len(txs))
	startTime := time.Now()

	// Generate batch ID
	batchID := generateBatchID(txs)

	// Parse transactions
	transactions := make([]service_registry.DeWSTransaction, 0, len(txs))
	txIDs := make([]string, 0, len(txs))

	// Process transactions (potentially in parallel)
	var wg sync.WaitGroup
	var processingLock sync.Mutex

	for _, txBytes := range txs {
		// Generate transaction ID
		txID := generateTxID(txBytes)
		txIDs = append(txIDs, txID)

		if s.config.ParallelExecution {
			wg.Add(1)
			go func(txb []byte, id string) {
				defer wg.Done()

				var tx service_registry.DeWSTransaction
				if err := json.Unmarshal(txb, &tx); err != nil {
					s.logger.Error("Failed to unmarshal transaction", "tx_id", id, "error", err)

					s.txStatusLock.Lock()
					s.txStatus[id] = &TransactionStatus{
						TxID:   id,
						Status: "failed",
						Error:  fmt.Sprintf("Failed to unmarshal: %v", err),
					}
					s.txStatusLock.Unlock()
					return
				}

				// Execute the transaction
				resp, err := s.executeTransaction(&tx)
				if err != nil {
					s.logger.Error("Failed to execute transaction", "tx_id", id, "error", err)

					s.txStatusLock.Lock()
					s.txStatus[id] = &TransactionStatus{
						TxID:   id,
						Status: "failed",
						Error:  fmt.Sprintf("Execution error: %v", err),
					}
					s.txStatusLock.Unlock()
					return
				}

				// Update transaction with response
				tx.Response = *resp

				processingLock.Lock()
				transactions = append(transactions, tx)
				processingLock.Unlock()

				// Update status to "processed"
				s.txStatusLock.Lock()
				s.txStatus[id] = &TransactionStatus{
					TxID:   id,
					Status: "processed",
				}
				s.txStatusLock.Unlock()
			}(txBytes, txID)
		} else {
			// Serial execution
			var tx service_registry.DeWSTransaction
			if err := json.Unmarshal(txBytes, &tx); err != nil {
				s.logger.Error("Failed to unmarshal transaction", "tx_id", txID, "error", err)
				continue
			}

			// Execute the transaction
			resp, err := s.executeTransaction(&tx)
			if err != nil {
				s.logger.Error("Failed to execute transaction", "tx_id", txID, "error", err)
				continue
			}

			// Update transaction with response
			tx.Response = *resp
			transactions = append(transactions, tx)
		}
	}

	if s.config.ParallelExecution {
		wg.Wait()
	}

	// Create the batch with processed transactions
	batch := &SequencerBatch{
		BatchID:        batchID,
		Transactions:   transactions,
		StateRoot:      calculateStateRoot(transactions),
		PrevStateRoot:  s.lastStateRoot,
		Timestamp:      time.Now(),
		SequencerID:    s.config.NodeID,
		TransactionIDs: txIDs,
		BatchNumber:    s.batchNumber,
	}

	s.batchNumber++
	s.lastStateRoot = batch.StateRoot

	// Store the batch
	s.batchesLock.Lock()
	s.batches[batchID] = batch
	s.batchesLock.Unlock()

	// Update transaction statuses
	s.txStatusLock.Lock()
	for _, txID := range txIDs {
		status, exists := s.txStatus[txID]
		if !exists || status.Status == "failed" {
			continue
		}

		s.txStatus[txID] = &TransactionStatus{
			TxID:         txID,
			Status:       "included",
			BatchID:      batchID,
			IncludedTime: time.Now(),
		}
	}
	s.txStatusLock.Unlock()

	processingTime := time.Since(startTime)
	s.logger.Info("Batch processed",
		"batch_id", batchID,
		"tx_count", len(txs),
		"successful", len(transactions),
		"processing_time_ms", processingTime.Milliseconds(),
		"batch_number", batch.BatchNumber,
	)
}

// executeTransaction executes a single transaction
func (s *Sequencer) executeTransaction(tx *service_registry.DeWSTransaction) (*service_registry.DeWSResponse, error) {
	// If the transaction already has a response, return it
	if tx.Response.StatusCode != 0 {
		return &tx.Response, nil
	}

	// Otherwise, generate a response
	return tx.Request.GenerateResponse(s.serviceRegistry)
}

// commitStateToL1 commits the current state to L1
func (s *Sequencer) commitStateToL1() error {
	// Get all uncommitted batches
	var uncommittedBatches []*SequencerBatch

	s.batchesLock.RLock()
	for _, batch := range s.batches {
		if !batch.L1Committed {
			uncommittedBatches = append(uncommittedBatches, batch)
		}
	}
	s.batchesLock.RUnlock()

	if len(uncommittedBatches) == 0 {
		s.logger.Info("No uncommitted batches to commit to L1")
		return nil
	}

	s.logger.Info("Committing state to L1", "batch_count", len(uncommittedBatches))

	// Create a state commitment
	commitment := map[string]interface{}{
		"sequencer_id":      s.config.NodeID,
		"batch_count":       len(uncommittedBatches),
		"batch_ids":         getBatchIDs(uncommittedBatches),
		"latest_state_root": hex.EncodeToString(s.lastStateRoot),
		"timestamp":         time.Now(),
		"latest_batch_num":  s.batchNumber - 1,
	}

	// Send to L1
	commitmentBytes, err := json.Marshal(commitment)
	if err != nil {
		return fmt.Errorf("failed to marshal commitment: %w", err)
	}

	// Create HTTP request to L1
	req, err := http.NewRequest("POST", s.config.L1Endpoint+"/api/l2-commitment", bytes.NewBuffer(commitmentBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := s.l1Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send commitment to L1: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("L1 returned non-success status: %d", resp.StatusCode)
	}

	// Parse response to get transaction hash
	var respData struct {
		TxHash string `json:"tx_hash"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Update batches as committed
	s.batchesLock.Lock()
	for _, batch := range uncommittedBatches {
		batch.L1Committed = true
		batch.L1TxHash = respData.TxHash
		s.batches[batch.BatchID] = batch
	}
	s.batchesLock.Unlock()

	// Update transaction statuses
	s.txStatusLock.Lock()
	for _, batch := range uncommittedBatches {
		for _, txID := range batch.TransactionIDs {
			status, exists := s.txStatus[txID]
			if !exists || status.Status == "failed" {
				continue
			}

			s.txStatus[txID] = &TransactionStatus{
				TxID:         txID,
				Status:       "committed",
				BatchID:      batch.BatchID,
				IncludedTime: status.IncludedTime,
				CommitTime:   time.Now(),
			}
		}
	}
	s.txStatusLock.Unlock()

	s.logger.Info("State committed to L1 successfully",
		"tx_hash", respData.TxHash,
		"batch_count", len(uncommittedBatches),
	)

	return nil
}

// Helper functions

// generateTxID generates a transaction ID
func generateTxID(txBytes []byte) string {
	hash := sha256.Sum256(txBytes)
	return hex.EncodeToString(hash[:])
}

// generateBatchID generates a batch ID
func generateBatchID(txs [][]byte) string {
	hasher := sha256.New()
	for _, tx := range txs {
		hasher.Write(tx)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// calculateStateRoot calculates a state root from transactions
func calculateStateRoot(txs []service_registry.DeWSTransaction) []byte {
	// In a real implementation, this would be a Merkle root or other cryptographic accumulator
	// For simplicity, we're just hashing all transaction IDs together
	hasher := sha256.New()
	for _, tx := range txs {
		txBytes, _ := json.Marshal(tx)
		hasher.Write(txBytes)
	}
	return hasher.Sum(nil)
}

// getBatchIDs returns the IDs of all batches
func getBatchIDs(batches []*SequencerBatch) []string {
	ids := make([]string, len(batches))
	for i, batch := range batches {
		ids[i] = batch.BatchID
	}
	return ids
}
