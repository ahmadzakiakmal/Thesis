package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

type AssetTransferLotteryContract struct {
	contractapi.Contract
}

// Node represents a network node that can participate in the consensus round
type Node struct {
	ID                    string `json:"id"`
	PublicKey             string `json:"publicKey"`
	RegistrationTimestamp string `json:"registrationTimestamp"`
	Active                bool   `json:"active"`
}

// BlockProposal represents a proposed block
type BlockProposal struct {
	NodeID    string `json:"nodeId"`
	BlockData string `json:"blockData"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

// ConsensusParams holds the parameter
type ConsensusParams struct {
	CommitteSizePercent int `json:"committeeSizePercent"`
	RotationBlockCount  int `json:"rotationBlockCount"`
	CurrentBlockHeight  int `json:"currentBlockHeight"`
}

func (rc *AssetTransferLotteryContract) InitLedger(
	ctx contractapi.TransactionContext,
	committeeSizePercent string,
	rotationBlockCount string) (string, error) {
	fmt.Println("🔵 Initializing lottery consensus ledger")

	// Parse and validate parameters
	size, err := strconv.Atoi(committeeSizePercent)
	if err != nil {
		return "", fmt.Errorf("failed to parse committee size: %v", err)
	}

	if size <= 0 || size > 100 {
		return "", fmt.Errorf("committee size must be a percentage value between 0 and 100")
	}

	blockCount, err := strconv.Atoi(rotationBlockCount)
	if err != nil {
		return "", fmt.Errorf("failed to parse block count rotation: %v", err)
	}

	if blockCount <= 0 {
		return "", fmt.Errorf("rotation block must be a positive number")
	}

	// Store consensus parameters
	err = ctx.GetStub().PutState("committeeSizePercent", []byte(committeeSizePercent))
	if err != nil {
		return "", err
	}

	err = ctx.GetStub().PutState("rotationBlockCount", []byte(rotationBlockCount))
	if err != nil {
		return "", err
	}

	err = ctx.GetStub().PutState("currentBlockCount", []byte("0"))
	if err != nil {
		return "", err
	}

	// Generate a random seed based on the transaction id of the proposal
	randomSeed := sha256.Sum256([]byte(ctx.GetStub().GetTxID()))
	err = ctx.GetStub().PutState("currentSeed", randomSeed[:])
	if err != nil {
		return "", err
	}

	// Initialize empty committee
	emptyCommittee, err := json.Marshal([]string{})
	if err != nil {
		return "", err
	}
	err = ctx.GetStub().PutState("currentCommittee", emptyCommittee)
	if err != nil {
		return "", err
	}

	fmt.Println("🟢 Initialization Complete")
	return "Ledger initialized successfully", nil
}

func (rc *AssetTransferLotteryContract) RegisterNode(
	ctx contractapi.TransactionContextInterface,
	nodeId string,
	publicKey string,
) (string, error) {
	// TODO: Q - Are Nodes equal to Peers?
	fmt.Println("🔵 Registering Node: ", nodeId)

	nodeAsBytes, err := ctx.GetStub().GetState(nodeId)
	if err != nil {
		return "", fmt.Errorf("failed to read node state: %v", err)
	}
	if nodeAsBytes != nil && len(nodeAsBytes) > 0 {
		return "", fmt.Errorf("node %s already exists", nodeId)
	}

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return "", fmt.Errorf("failed to get transaction timestamp: %v", err)
	}

	// Store node information
	node := Node{
		ID:                    nodeId,
		PublicKey:             publicKey,
		RegistrationTimestamp: txTimestamp.String(),
		Active:                true,
	}

	nodeJSON, err := json.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("failed marshaling node data into JSON: %v", err)
	}

	return string(nodeJSON), nil
}
