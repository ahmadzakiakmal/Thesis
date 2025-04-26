package main

import (
	"fmt"
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

func main() {
	l2Url := "http://127.0.0.1:4000"
	requestClient := client.NewHTTPClient(l2Url)
	opts := &client.RequestOptions{
		Headers: map[string]string{
			"Accept":        "*/*",
			"Cache-Control": "no-cache",
			"Connection":    "keep-alive",
			"User-Agent":    "PostmanRuntime/7.43.3", // Match Postman's user agent
		},
		Timeout: 10 * time.Second,
	}

	// 1. Create Test Package
	resp, err := requestClient.POST("/session/test-package", nil, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	var testPackageResponse testPackageResponse
	client.UnmarshalBody(resp, &testPackageResponse)
	packageId := testPackageResponse.Body.PackageID
	fmt.Printf("PackageID : %s\n", packageId)

	// 2. Start Session
	body := map[string]interface{}{
		"operator_id": operatorID,
	}
	resp, err = requestClient.POST("/session/start", body, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	var startSessionResponse startSessionResponse
	client.UnmarshalBody(resp, &startSessionResponse)
	sessionID := startSessionResponse.Body.SessionID
	fmt.Printf("SessionID : %s\n", sessionID)

	// 3. Scan Package
	endpoint := fmt.Sprintf("/session/%s/scan/%s", sessionID, packageId)
	_, err = requestClient.GET(endpoint, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Package scan success")

	// 4. Validate Package
	endpoint = fmt.Sprintf("/session/%s/validate", sessionID)
	body = map[string]interface{}{
		"package_id": packageId,
		"signature":  "any",
	}
	_, err = requestClient.POST(endpoint, body, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Package validation success")

	// 5. Quality Check
	endpoint = fmt.Sprintf("/session/%s/qc", sessionID)
	// simulate a passed quality check
	body = map[string]interface{}{
		"passed": true,
		"issues": []string{"all good"},
	}
	_, err = requestClient.POST(endpoint, body, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("QC request successful")

	// 6. Label Package
	endpoint = fmt.Sprintf("/session/%s/label", sessionID)
	body = map[string]interface{}{
		"destination": destination,
		"priority":    priority,
		"courier_id":  courierID,
	}
	_, err = requestClient.POST(endpoint, body, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Package labelling successful")

	// 7. Commit
	endpoint = fmt.Sprintf("/session/%s/commit", sessionID)
	// simulate a passed quality check
	body = map[string]interface{}{
		"operator_id":        operatorID,
		"package_id":         packageId,
		"supplier_signature": "any",
		"destination":        destination,
		"priority":           priority,
		"courier_id":         courierID,
	}
	resp, err = requestClient.POST(endpoint, body, opts)
	if err != nil {
		fmt.Println(err)
	}
	var commitSessionResponse commitSessionResponse
	client.UnmarshalBody(resp, &commitSessionResponse)
	// fmt.Println(string(resp.Body))
	blockHeight := commitSessionResponse.Body.L1.BlockHeight
	if blockHeight != 0 {
		fmt.Printf("Session %s, committed successfully to L1, block height %d\n", sessionID, blockHeight)
	}
}
