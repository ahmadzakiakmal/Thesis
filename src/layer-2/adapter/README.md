# DeWS Layer 2 Adapter

This adapter implementation connects a Rollkit-based Layer 2 system with the DeWS Layer 1 Byzantine Fault-tolerant network. It processes transactions in batches and submits them to the Layer 1 network for consensus and validation.

## Architecture

The Layer 2 adapter serves as both:
1. A Data Availability (DA) layer for the Rollkit rollup
2. A transaction processor that batches transactions and forwards them to Layer 1

## Key Features

1. **JSONRPC Compatibility**: Implements the Data Availability (DA) interface required by Rollkit
2. **Transaction Batching**: Groups transactions for efficient processing
3. **Format Conversion**: Converts Layer 2 transactions to the proper format for Layer 1
4. **Status Tracking**: Maintains transaction status from submission to confirmation
5. **Configurable Batch Sizes**: Allows tuning for optimal throughput

## Implementation Details

### Components

1. **BatchProcessor**: Manages transaction batching and submission to Layer 1
2. **TransactionStore**: In-memory storage for tracking transactions
3. **HTTP Server**: Handles incoming Layer 2 requests
4. **JSONRPC Handler**: Implements the DA protocol for Rollkit compatibility

### Transaction Flow

1. Layer 2 nodes submit transactions to the adapter
2. The adapter batches transactions based on configurable parameters
3. When a batch is ready (based on size or time), it's processed:
   - Each transaction is formatted for Layer 1
   - Transactions are submitted to Layer 1 endpoints
   - Responses are stored with transaction records
4. Status tracking is maintained throughout the process

## Configuration Options

The adapter supports several configuration options via command-line flags:

- `--listen`: Address to listen on for Layer 2 requests (default: `:7980`)
- `--layer1`: Layer 1 API endpoint (default: `http://localhost:5000`)
- `--da-mode`: Data availability mode (`mock` or `celestia`)
- `--node-id`: Node ID for this adapter
- `--batch-interval`: Time interval for batching transactions (default: `5s`)
- `--batch-size`: Maximum transactions in a batch (default: `100`)

## API Endpoints

- `/`: Status page with adapter information
- `/tx`: Submit a new transaction
- `/status/{txID}`: Check transaction status
- `/batch`: Manually trigger batch processing

## JSONRPC Methods

The adapter implements the following JSONRPC methods required by Rollkit:

- `da.MaxBlobSize`: Returns the maximum blob size supported
- `da.Submit`: Submits data to the DA layer
- `da.GetIDs`: Retrieves transaction IDs for a given block height
- `da.Get`: Retrieves transactions by their IDs
- `status`: Returns the current status of the adapter

## Performance Benefits

Based on the initial testing from "Week 11 Progress", the Layer 2 implementation offers approximately 2x throughput improvement compared to a 4-node Layer 1 system with 100-200 concurrent requests.

## Setup Instructions

1. Build the adapter:
   ```bash
   cd src/layer-2
   go build -o adapter adapter/main.go
   ```

2. Run the adapter:
   ```bash
   ./adapter --layer1=http://localhost:5000 --listen=:7980
   ```

3. Configure Rollkit to use this adapter as its DA layer:
   ```bash
   # In your rollkit configuration:
   --rollkit.da_address="http://localhost:7980"
   ```

## Next Steps for Development

1. **Persistence**: Add database storage for transactions
2. **Authentication**: Add secure authentication between layers
3. **Celestia Integration**: Support for production Celestia DA layer
4. **Metrics and Monitoring**: Add Prometheus metrics for performance tracking
5. **Load Testing**: Comprehensive performance evaluation