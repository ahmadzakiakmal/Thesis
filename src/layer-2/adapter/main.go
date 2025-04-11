package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync/atomic"
)

// Simple in-memory block storage
var currentHeight uint64 = 0
var blobs = make(map[uint64][][]byte)

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

func main() {
	// Handle all paths
	http.HandleFunc("/", handleRequest)

	fmt.Println("Starting mock DA server on :7980")
	log.Fatal(http.ListenAndServe(":7980", nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Log all requests
	fmt.Printf("Received %s request to %s\n", r.Method, r.URL.Path)

	if r.Method == "POST" {
		// Read the request body
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}

		fmt.Printf("Request body: %s\n", string(body))

		// Try to parse as JSONRPC
		var req JSONRPCRequest
		if err := json.Unmarshal(body, &req); err == nil && req.Method != "" {
			handleJSONRPC(w, req)
			return
		}

		// If not JSONRPC, handle as a simple POST request
		// Always return success for any POST request
		fmt.Println("Handling as simple POST request")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result": "success"}`))
	} else if r.Method == "GET" {
		// Handle GET requests
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Fake DA Server is running"))
	} else {
		// Return 200 OK for all request methods to avoid compatibility issues
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "success"}`))
	}
}

func handleJSONRPC(w http.ResponseWriter, req JSONRPCRequest) {
	fmt.Printf("Handling JSONRPC request: Method=%s, ID=%v\n", req.Method, req.ID)
	fmt.Printf("Params: %s\n", string(req.Params))

	// Handle request based on method
	switch req.Method {
	case "da.MaxBlobSize":
		// Return a uint64 value directly, not an array
		maxBlobSize := uint64(1024 * 1024) // 1MB
		sendSuccessResponse(w, req.ID, maxBlobSize)

	case "da.Submit":
		// For da.Submit, try different parameter formats
		// First, try parsing as array of arrays (blobs)
		var blobsArray [][]byte
		if err := json.Unmarshal(req.Params, &blobsArray); err == nil {
			fmt.Printf("Successfully parsed params as blob array, received %d blobs\n", len(blobsArray))

			// Increment height for new blocks
			newHeight := atomic.AddUint64(&currentHeight, 1)

			// Store blobs for this height
			blobs[newHeight] = blobsArray

			// Create an ID for each blob (using height as ID)
			var ids [][]byte
			idBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(idBytes, newHeight)
			ids = append(ids, idBytes)

			// Return the IDs
			sendSuccessResponse(w, req.ID, ids)
			return
		}

		// If that fails, try parsing as struct with Blobs field
		var paramsStruct struct {
			Blobs     [][]byte `json:"blobs"`
			GasPrice  float64  `json:"gas_price"`
			Namespace string   `json:"namespace"`
		}
		if err := json.Unmarshal(req.Params, &paramsStruct); err == nil {
			fmt.Printf("Successfully parsed params as struct, received %d blobs\n", len(paramsStruct.Blobs))

			// Increment height for new blocks
			newHeight := atomic.AddUint64(&currentHeight, 1)

			// Store blobs for this height
			blobs[newHeight] = paramsStruct.Blobs

			// Create an ID for each blob (using height as ID)
			var ids [][]byte
			idBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(idBytes, newHeight)
			ids = append(ids, idBytes)

			// Return the IDs
			sendSuccessResponse(w, req.ID, ids)
			return
		}

		// If both fail, try a different approach - assume it's an array of parameters
		var params []interface{}
		if err := json.Unmarshal(req.Params, &params); err == nil {
			fmt.Printf("Successfully parsed params as array of parameters, length: %d\n", len(params))

			// Try to extract blobs from first parameter
			var blobsData [][]byte
			if len(params) > 0 {
				// Convert first parameter to JSON
				blobsJSON, err := json.Marshal(params[0])
				if err == nil {
					if err := json.Unmarshal(blobsJSON, &blobsData); err == nil {
						fmt.Printf("Successfully extracted %d blobs from first parameter\n", len(blobsData))

						// Increment height for new blocks
						newHeight := atomic.AddUint64(&currentHeight, 1)

						// Store blobs for this height
						blobs[newHeight] = blobsData

						// Create an ID for each blob (using height as ID)
						var ids [][]byte
						idBytes := make([]byte, 8)
						binary.LittleEndian.PutUint64(idBytes, newHeight)
						ids = append(ids, idBytes)

						// Return the IDs
						sendSuccessResponse(w, req.ID, ids)
						return
					}
				}
			}
		}

		// If all parsing attempts fail, log the issue and return a simple response
		fmt.Printf("Failed to parse params for da.Submit: %s\n", string(req.Params))
		// Just increment height and return a valid ID
		newHeight := atomic.AddUint64(&currentHeight, 1)
		idBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(idBytes, newHeight)
		sendSuccessResponse(w, req.ID, [][]byte{idBytes})

	case "status":
		result := map[string]interface{}{
			"node_info": map[string]interface{}{
				"network": "test-network",
			},
			"sync_info": map[string]interface{}{
				"latest_block_height": currentHeight,
			},
		}
		sendSuccessResponse(w, req.ID, result)

	case "da.GetIDs":
		// Parse the params to get height
		var height uint64
		if err := json.Unmarshal(req.Params, &height); err == nil {
			// Simple uint64 parameter
			fmt.Printf("Parsed height directly: %d\n", height)
			handleGetIDs(w, req.ID, height)
			return
		}

		// Try array of parameters
		var params []interface{}
		if err := json.Unmarshal(req.Params, &params); err == nil && len(params) > 0 {
			// Try to extract height from first parameter
			if heightFloat, ok := params[0].(float64); ok {
				fmt.Printf("Extracted height from array: %f\n", heightFloat)
				handleGetIDs(w, req.ID, uint64(heightFloat))
				return
			}
		}

		// Try struct with Height field
		var paramsStruct struct {
			Height    uint64 `json:"height"`
			Namespace string `json:"namespace"`
		}
		if err := json.Unmarshal(req.Params, &paramsStruct); err == nil {
			fmt.Printf("Parsed height from struct: %d\n", paramsStruct.Height)
			handleGetIDs(w, req.ID, paramsStruct.Height)
			return
		}

		// If all parsing attempts fail, log the issue and return empty IDs
		fmt.Printf("Failed to parse params for da.GetIDs: %s\n", string(req.Params))
		sendSuccessResponse(w, req.ID, map[string]interface{}{
			"ids": []string{},
		})

	case "da.Get":
		// Similar approach for da.Get
		var ids [][]byte
		if err := json.Unmarshal(req.Params, &ids); err == nil {
			fmt.Printf("Successfully parsed params as ID array, received %d IDs\n", len(ids))
			handleGet(w, req.ID, ids)
			return
		}

		// Try array of parameters
		var params []interface{}
		if err := json.Unmarshal(req.Params, &params); err == nil && len(params) > 0 {
			// Try to extract IDs from first parameter
			idsJSON, err := json.Marshal(params[0])
			if err == nil {
				if err := json.Unmarshal(idsJSON, &ids); err == nil {
					fmt.Printf("Successfully extracted %d IDs from first parameter\n", len(ids))
					handleGet(w, req.ID, ids)
					return
				}
			}
		}

		// Try struct with IDs field
		var paramsStruct struct {
			IDs       [][]byte `json:"ids"`
			Namespace string   `json:"namespace"`
		}
		if err := json.Unmarshal(req.Params, &paramsStruct); err == nil {
			fmt.Printf("Successfully parsed params as struct, received %d IDs\n", len(paramsStruct.IDs))
			handleGet(w, req.ID, paramsStruct.IDs)
			return
		}

		// If all parsing attempts fail, log the issue and return empty blobs
		fmt.Printf("Failed to parse params for da.Get: %s\n", string(req.Params))
		sendSuccessResponse(w, req.ID, [][]byte{})

	default:
		// For unknown methods, log and return a simple response
		fmt.Printf("Unknown method: %s\n", req.Method)
		sendSuccessResponse(w, req.ID, 0)
	}
}

func handleGetIDs(w http.ResponseWriter, id interface{}, height uint64) {
	// Check if we have blobs for this height
	if _, ok := blobs[height]; !ok {
		// No blocks at this height
		sendSuccessResponse(w, id, map[string]interface{}{
			"ids": []string{},
		})
		return
	}

	// Create IDs for the blobs
	var ids [][]byte
	idBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(idBytes, height)
	ids = append(ids, idBytes)

	// Return the IDs
	sendSuccessResponse(w, id, map[string]interface{}{
		"ids": ids,
	})
}

func handleGet(w http.ResponseWriter, id interface{}, ids [][]byte) {
	// Return blobs for the requested IDs
	var retrievedBlobs [][]byte
	for _, idBytes := range ids {
		if len(idBytes) >= 8 {
			height := binary.LittleEndian.Uint64(idBytes)
			if heightBlobs, ok := blobs[height]; ok {
				retrievedBlobs = append(retrievedBlobs, heightBlobs...)
			}
		}
	}

	// Return the blobs
	sendSuccessResponse(w, id, retrievedBlobs)
}

func sendSuccessResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func sendErrorResponse(w http.ResponseWriter, id interface{}, code int, message string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
		},
		ID: id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // Always 200 OK for JSONRPC errors
	json.NewEncoder(w).Encode(response)
}
