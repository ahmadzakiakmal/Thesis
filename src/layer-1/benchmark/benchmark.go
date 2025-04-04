package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// User represents the user data structure
type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	// Command line flags
	url := flag.String("url", "http://localhost:5000/api/customers", "API endpoint URL")
	method := flag.String("method", "GET", "HTTP method (GET, POST, PUT, DELETE)")
	numRequests := flag.Int("n", 10, "Number of requests to make")
	data := flag.String("data", "", "Request body for POST/PUT requests")
	header := flag.String("header", "Content-Type: application/json", "HTTP header")
	verbose := flag.Bool("verbose", false, "Print detailed output for each request")
	randomizeEmails := flag.Bool("randomize", false, "Generate random emails for each request (POST only)")
	namePrefix := flag.String("name", "TestUser", "Base name prefix for generated users")
	emailDomain := flag.String("domain", "example.com", "Email domain for generated emails")
	withConsensus := flag.Bool("consensus", true, "Enable consensus for requests (true/false)")
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

	// Ensure we have Content-Type for JSON if needed
	if (*method == "POST" || *method == "PUT") && *randomizeEmails {
		headerMap["Content-Type"] = "application/json"
	}

	fmt.Println("========================================================")
	fmt.Printf("Benchmarking API: %s\n", *url)
	fmt.Printf("Method: %s\n", *method)
	fmt.Printf("Number of requests: %d\n", *numRequests)
	fmt.Printf("Consensus: %v\n", *withConsensus)
	if *randomizeEmails && (*method == "POST" || *method == "PUT") {
		fmt.Println("Using randomized emails for each request")
		fmt.Printf("Name prefix: %s\n", *namePrefix)
		fmt.Printf("Email domain: %s\n", *emailDomain)
	}
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

	// Prepare the base URL with consensus parameter
	baseURL := *url
	if strings.Contains(baseURL, "?") {
		baseURL = baseURL + "&consensus=" + consensusValue(*withConsensus)
	} else {
		baseURL = baseURL + "?consensus=" + consensusValue(*withConsensus)
	}

	// Make requests
	for i := 1; i <= *numRequests; i++ {
		var req *http.Request
		var err error
		var bodyData string

		// Prepare the request body
		if *randomizeEmails && (*method == "POST" || *method == "PUT") {
			// Generate random identifier
			randomID := generateRandomID(8)

			// Create user with randomized email
			user := User{
				Name:  fmt.Sprintf("%s%d", *namePrefix, i),
				Email: fmt.Sprintf("%s.%s@%s", *namePrefix, randomID, *emailDomain),
			}

			// Convert to JSON
			jsonData, err := json.Marshal(user)
			if err != nil {
				fmt.Printf("Error creating JSON: %v\n", err)
				continue
			}

			bodyData = string(jsonData)

			if *verbose {
				fmt.Printf("Request %d body: %s\n", i, bodyData)
			}
		} else {
			bodyData = *data
		}

		// Create the request
		if *method == "GET" || *method == "DELETE" {
			req, err = http.NewRequest(*method, baseURL, nil)
		} else {
			req, err = http.NewRequest(*method, baseURL, bytes.NewBufferString(bodyData))
		}

		if err != nil {
			fmt.Printf("Error creating request: %v\n", err)
			continue
		}

		// Add headers
		for key, value := range headerMap {
			req.Header.Set(key, value)
		}

		// Measure response time
		startTime := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Request %d: Error - %v\n", i, err)
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
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
			fmt.Printf("Request %d: %.2f ms (Status: %s)\n", i, timeMs, resp.Status)
			fmt.Printf("Response: %s\n", string(body))
		} else {
			fmt.Printf("Request %d: %.2f ms (Status: %s)\n", i, timeMs, resp.Status)
		}
	}

	// Skip stats if no successful requests
	if len(times) == 0 {
		fmt.Println("========================================================")
		fmt.Println("No successful requests to calculate statistics")
		fmt.Println("========================================================")
		return
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

	// Display consensus status in the results
	if *withConsensus {
		fmt.Println("Consensus: Enabled")
	} else {
		fmt.Println("Consensus: Disabled")
	}
	fmt.Println("Use --consensus=false or --consensus=true to toggle consensus")
	fmt.Println("========================================================")
}

// generateRandomID creates a random string of specified length
func generateRandomID(length int) string {
	bytes := make([]byte, length/2+1)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// consensusValue converts boolean to string value for URL parameter
func consensusValue(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}
