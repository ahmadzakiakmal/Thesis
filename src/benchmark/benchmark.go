package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/benchmark/client"
)

type testPackageResponse struct {
	Body struct {
		PackageID string `json:"package_id"`
	} `json:"body"`
}

type startSessionResponse struct {
	Body struct {
		SessionID string `json:"id"`
	} `json:"body"`
}

type commitSessionResponse struct {
	Body struct {
		L1 struct {
			BlockHeight int64 `json:"BlockHeight"`
		} `json:"l1"`
	} `json:"body"`
}

const (
	operatorID  = "OPR-001"
	destination = "CUSTOMER A"
	courierID   = "COU-001"
	priority    = "standard"
)

type RequestResult struct {
	Name        string
	Method      string
	Endpoint    string
	Layer       string
	Latency     time.Duration
	BlockHeight int64
}

func main() {
	l1Nodes := flag.Int("l1", 4, "Number of Layer 1 nodes")
	l2Nodes := flag.Int("l2", 1, "Number of Layer 2 nodes")
	iterations := flag.Int("n", 1, "Number of iterations to run")
	useIPv6 := flag.Bool("ipv6", false, "Use IPv6 localhost (::1) instead of IPv4")
	flag.Parse()

	l2Url := "http://127.0.0.1:4000"
	if *useIPv6 {
		l2Url = "http://[::1]:4000"
	}

	filename := fmt.Sprintf("benchmark_n_%d_l1_%d_l2_%d.csv", *iterations, *l1Nodes, *l2Nodes)
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating CSV file: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Iteration", "Step", "Method", "Endpoint", "Layer", "Latency_ms", "BlockHeight"}
	if err := writer.Write(header); err != nil {
		fmt.Printf("Error writing CSV header: %v\n", err)
		return
	}

	requestClient := client.NewHTTPClient(l2Url)
	opts := &client.RequestOptions{
		Headers: map[string]string{
			"Accept":        "*/*",
			"Cache-Control": "no-cache",
			"Connection":    "keep-alive",
			"User-Agent":    "PostmanRuntime/7.43.3",
		},
		Timeout: 10 * time.Second,
	}

	for i := 0; i < *iterations; i++ {
		fmt.Printf("\n[Iteration %d/%d]\n", i+1, *iterations)
		results := runBenchmark(requestClient, opts)

		for _, result := range results {
			record := []string{
				strconv.Itoa(i + 1),
				result.Name,
				result.Method,
				result.Endpoint,
				result.Layer,
				strconv.FormatInt(result.Latency.Milliseconds(), 10),
				strconv.FormatInt(result.BlockHeight, 10),
			}

			if err := writer.Write(record); err != nil {
				fmt.Printf("Error writing record to CSV: %v\n", err)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("\nBenchmark complete. Results saved to %s\n", filename)
}

func runBenchmark(requestClient *client.HTTPClient, opts *client.RequestOptions) []RequestResult {
	var results []RequestResult
	totalStart := time.Now()

	// 1. Create Test Package
	start := time.Now()
	resp, err := requestClient.POST("/session/test-package", nil, opts)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Println(err)
		return results
	}
	var testPackageResponse testPackageResponse
	client.UnmarshalBody(resp, &testPackageResponse)
	packageId := testPackageResponse.Body.PackageID
	fmt.Printf("PackageID : %s [Delay: %v]\n", packageId, elapsed)

	results = append(results, RequestResult{
		Name:     "Create Package",
		Method:   "POST",
		Endpoint: "/session/test-package",
		Layer:    "L2",
		Latency:  elapsed,
	})

	// 2. Start Session
	time.Sleep(100 * time.Millisecond)
	start = time.Now()
	body := map[string]interface{}{
		"operator_id": operatorID,
	}
	resp, err = requestClient.POST("/session/start", body, opts)
	elapsed = time.Since(start)
	if err != nil {
		fmt.Println(err)
		return results
	}
	var startSessionResponse startSessionResponse
	client.UnmarshalBody(resp, &startSessionResponse)
	sessionID := startSessionResponse.Body.SessionID
	fmt.Printf("SessionID : %s [Delay: %v]\n", sessionID, elapsed)

	results = append(results, RequestResult{
		Name:     "Start Session",
		Method:   "POST",
		Endpoint: "/session/start",
		Layer:    "L2",
		Latency:  elapsed,
	})

	// 3. Scan Package
	time.Sleep(100 * time.Millisecond)
	start = time.Now()
	endpoint := fmt.Sprintf("/session/%s/scan/%s", sessionID, packageId)
	_, err = requestClient.GET(endpoint, opts)
	elapsed = time.Since(start)
	if err != nil {
		fmt.Println(err)
		return results
	}
	fmt.Printf("Package scan success [Delay: %v]\n", elapsed)

	results = append(results, RequestResult{
		Name:     "Scan Package",
		Method:   "GET",
		Endpoint: "session/:id/scan/:packageId",
		Layer:    "L2",
		Latency:  elapsed,
	})

	// 4. Validate Package
	time.Sleep(100 * time.Millisecond)
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/validate", sessionID)
	body = map[string]interface{}{
		"package_id": packageId,
		"signature":  "any",
	}
	_, err = requestClient.POST(endpoint, body, opts)
	elapsed = time.Since(start)
	if err != nil {
		fmt.Println(err)
		return results
	}
	fmt.Printf("Package validation success [Delay: %v]\n", elapsed)

	results = append(results, RequestResult{
		Name:     "Validate Package",
		Method:   "POST",
		Endpoint: "session/:id/validate",
		Layer:    "L2",
		Latency:  elapsed,
	})

	// 5. Quality Check
	time.Sleep(100 * time.Millisecond)
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/qc", sessionID)
	body = map[string]interface{}{
		"passed": true,
		"issues": []string{"all good"},
	}
	_, err = requestClient.POST(endpoint, body, opts)
	elapsed = time.Since(start)
	if err != nil {
		fmt.Println(err)
		return results
	}
	fmt.Printf("QC request successful [Delay: %v]\n", elapsed)

	results = append(results, RequestResult{
		Name:     "Quality Check",
		Method:   "POST",
		Endpoint: "session/:id/qc",
		Layer:    "L2",
		Latency:  elapsed,
	})

	// 6. Label Package
	time.Sleep(100 * time.Millisecond)
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/label", sessionID)
	body = map[string]interface{}{
		"destination": destination,
		"priority":    priority,
		"courier_id":  courierID,
	}
	_, err = requestClient.POST(endpoint, body, opts)
	elapsed = time.Since(start)
	if err != nil {
		fmt.Println(err)
		return results
	}
	fmt.Printf("Package labelling successful [Delay: %v]\n", elapsed)

	results = append(results, RequestResult{
		Name:     "Label Package",
		Method:   "POST",
		Endpoint: "session/:id/label",
		Layer:    "L2",
		Latency:  elapsed,
	})

	// 7. Commit
	time.Sleep(100 * time.Millisecond)
	start = time.Now()
	endpoint = fmt.Sprintf("/commit/%s", sessionID)
	body = map[string]interface{}{
		"operator_id":        operatorID,
		"package_id":         packageId,
		"supplier_signature": "any",
		"destination":        destination,
		"priority":           priority,
		"courier_id":         courierID,
	}
	resp, err = requestClient.POST(endpoint, body, opts)
	elapsed = time.Since(start)
	if err != nil {
		fmt.Println(err)
		return results
	}
	var commitSessionResponse commitSessionResponse
	client.UnmarshalBody(resp, &commitSessionResponse)
	blockHeight := commitSessionResponse.Body.L1.BlockHeight
	fmt.Printf("Session %s, committed successfully to L1, block height %d [Delay: %v]\n", sessionID, blockHeight, elapsed)

	results = append(results, RequestResult{
		Name:        "Commit Session",
		Method:      "POST",
		Endpoint:    "commit/:id",
		Layer:       "L1+L2",
		Latency:     elapsed,
		BlockHeight: blockHeight,
	})

	totalElapsed := time.Since(totalStart)
	fmt.Printf("\nTotal workflow execution time: %v\n", totalElapsed)

	results = append(results, RequestResult{
		Name:     "Complete Workflow",
		Method:   "WORKFLOW",
		Endpoint: "complete-workflow",
		Layer:    "TOTAL",
		Latency:  totalElapsed,
	})

	return results
}
