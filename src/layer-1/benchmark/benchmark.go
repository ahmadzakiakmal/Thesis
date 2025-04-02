package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

func main() {
	// Command line flags
	url := flag.String("url", "http://localhost:5000/api/customers", "API endpoint URL")
	method := flag.String("method", "GET", "HTTP method (GET, POST, PUT, DELETE)")
	numRequests := flag.Int("n", 10, "Number of requests to make")
	data := flag.String("data", "", "Request body for POST/PUT requests")
	header := flag.String("header", "Content-Type: application/json", "HTTP header")
	verbose := flag.Bool("verbose", false, "Print detailed output for each request")
	flag.Parse()

	// Parse headers
	headerMap := make(map[string]string)
	headers := strings.Split(*header, ",")
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	fmt.Println("========================================================")
	fmt.Printf("Benchmarking API: %s\n", *url)
	fmt.Printf("Method: %s\n", *method)
	fmt.Printf("Number of requests: %d\n", *numRequests)
	fmt.Println("========================================================")

	// Create HTTP client
	client := &http.Client{
		Timeout: time.Second * 30,
	}

	// Store times
	var times []float64
	totalTime := 0.0
	minTime := math.MaxFloat64
	maxTime := 0.0

	// Make requests
	for i := 1; i <= *numRequests; i++ {
		var req *http.Request
		var err error

		// Create request based on method
		if *method == "GET" {
			req, err = http.NewRequest("GET", *url, nil)
		} else {
			req, err = http.NewRequest(*method, *url, bytes.NewBufferString(*data))
		}
		if err != nil {
			fmt.Printf("Error creating request: %v\n", err)
			continue
		}

		// Add headers
		for key, value := range headerMap {
			req.Header.Add(key, value)
		}

		// Measure response time
		startTime := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Request %d: Error - %v\n", i, err)
			continue
		}

		// Read and discard response body
		_, err = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("Error reading response: %v\n", err)
		}

		duration := time.Since(startTime)
		timeMs := float64(duration.Milliseconds())
		times = append(times, timeMs)

		// Update stats
		totalTime += timeMs
		if timeMs < minTime {
			minTime = timeMs
		}
		if timeMs > maxTime {
			maxTime = timeMs
		}

		if *verbose {
			fmt.Printf("Request %d: %v ms (Status: %s)\n", i, timeMs, resp.Status)
		} else {
			fmt.Printf("Request %d: %.2f ms\n", i, timeMs)
		}
	}

	// Calculate stats
	avgTime := totalTime / float64(len(times))

	// Calculate median and percentiles
	sort.Float64s(times)
	var median float64
	if len(times)%2 == 0 {
		median = (times[len(times)/2-1] + times[len(times)/2]) / 2
	} else {
		median = times[len(times)/2]
	}

	// Calculate 95th percentile
	p95Index := int(math.Ceil(float64(len(times))*0.95)) - 1
	if p95Index < 0 {
		p95Index = 0
	}
	p95 := times[p95Index]

	// Print results
	fmt.Println("========================================================")
	fmt.Println("Results:")
	fmt.Printf("Total requests:     %d\n", len(times))
	fmt.Printf("Average time:       %.2f ms\n", avgTime)
	fmt.Printf("Minimum time:       %.2f ms\n", minTime)
	fmt.Printf("Maximum time:       %.2f ms\n", maxTime)
	fmt.Printf("Median time:        %.2f ms\n", median)
	fmt.Printf("95th percentile:    %.2f ms\n", p95)
	fmt.Println("========================================================")

	// Check if consensus was used
	if strings.Contains(*url, "consensus=0") {
		fmt.Println("Consensus: Disabled")
	} else {
		fmt.Println("Consensus: Enabled (default)")
		fmt.Println("To benchmark without consensus, add '?consensus=0' to the URL")
	}
	fmt.Println("========================================================")
}
