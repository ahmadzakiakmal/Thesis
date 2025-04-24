package srvreg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-1/repository"
)

var defaultHeaders = map[string]string{"Content-Type": "application/json"}

type startSessionHandlerBody struct {
	OperatorID string `json:"operator_id"`
}

func (sr *ServiceRegistry) CreateSessionHandler(req *Request) (*Response, error) {
	sessionID := fmt.Sprintf("SESSION-%s", req.RequestID)
	var body startSessionHandlerBody
	err := json.Unmarshal([]byte(req.Body), &body)
	if err != nil {
		sr.logger.Info("Failed to parse body", "error", err.Error())
		return &Response{
			StatusCode: http.StatusUnprocessableEntity,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Failed to create session: %s"}`, err.Error()),
		}, err
	}

	operatorID := body.OperatorID
	if operatorID == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"operator ID is required"}`,
		}, err
	}

	session, dbErr := sr.repository.CreateSession(sessionID, operatorID)
	if dbErr != nil {
		switch dbErr.Code {
		case repository.PgErrForeignKeyViolation: // PostgreSQL foreign key violation
			return &Response{
				StatusCode: http.StatusBadRequest,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("foreign key violation: %s", dbErr.Message)
		case repository.PgErrUniqueViolation: // PostgreSQL unique violation
			return &Response{
				StatusCode: http.StatusConflict,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("unique violation: %s", dbErr.Message)
		default:
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    defaultHeaders,
				Body:       `{"error":"Internal server error"}`,
			}, nil
		}
	}

	return &Response{
		StatusCode: http.StatusCreated, // or http.StatusOK
		Headers:    defaultHeaders,
		Body:       fmt.Sprintf(`{"message":"Session generated","id":"%s"}`, session.ID),
	}, nil
}

type scanPackageHandlerBody struct {
	PackageID string `json:"package_id"`
}

func (sr *ServiceRegistry) ScanPackageHandler(req *Request) (*Response, error) {
	var body scanPackageHandlerBody
	err := json.Unmarshal([]byte(req.Body), &body)
	if err != nil {
		sr.logger.Info("Failed to parse body", "error", err.Error())
		return &Response{
			StatusCode: http.StatusUnprocessableEntity,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid body format: %s"}`, err.Error()),
		}, fmt.Errorf("invalid body format")
	}

	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}
	sessionID := pathParts[2]

	if body.PackageID == "" {
		return nil, fmt.Errorf("package_id is required")
	}

	pkg, dbErr := sr.repository.ScanPackage(sessionID, body.PackageID)
	if dbErr != nil {
		switch dbErr.Code {
		case "ENTITY_NOT_FOUND":
			return &Response{
				StatusCode: http.StatusNotFound,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
			}, fmt.Errorf("entity not found: %s", dbErr.Message)
		default:
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    defaultHeaders,
				Body:       `{"error":"Internal server error"}`,
			}, nil
		}
	}

	// Format the items for the response
	var expectedContents []map[string]interface{}
	for _, item := range pkg.Items {
		expectedContents = append(expectedContents, map[string]interface{}{
			"item_id": item.ID,
			"item":    item.Description,
			"qty":     item.Quantity,
		})
	}

	// Convert to JSON
	contentsJSON, err := json.Marshal(expectedContents)
	if err != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Failed to process item data"}`,
		}, nil
	}

	// Get supplier name
	supplierName := "Unknown Supplier"
	if pkg.Supplier != nil {
		supplierName = pkg.Supplier.Name
	}

	// Build the response
	response := fmt.Sprintf(`{
			"status": 200,
			"source": "%s",
			"package_id": "%s",
			"expected_contents": %s,
			"supplier_signature": "%s",
			"next_step": "validate"
	}`, supplierName, pkg.ID, string(contentsJSON), pkg.Signature)

	// Remove whitespace for valid JSON
	response = strings.Replace(strings.Replace(strings.Replace(response, "\n", "", -1), "    ", "", -1), "\t", "", -1)

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body:       response,
	}, nil
}

type validatePackageHandlerBody struct {
	Signature string `json:"signature"`
	PackageID string `json:"package_id"`
}

func (sr *ServiceRegistry) ValidatePackageHandler(req *Request) (*Response, error) {
	var body validatePackageHandlerBody
	err := json.Unmarshal([]byte(req.Body), &body)
	if err != nil {
		sr.logger.Info("Failed to parse body", "error", err.Error())
		return &Response{
			StatusCode: http.StatusUnprocessableEntity,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid body format: %s"}`, err.Error()),
		}, fmt.Errorf("invalid body format")
	}
	if body.Signature == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"signature is required"}`,
		}, err
	}
	if body.PackageID == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"package_id is required"}`,
		}, err
	}

	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}
	sessionID := pathParts[2]

	pkg, dbErr := sr.repository.ValidatePackage(body.Signature, body.PackageID, sessionID)
	if dbErr != nil {
		switch dbErr.Code {
		case "ENTITY_NOT_FOUND":
			return &Response{
				StatusCode: http.StatusNotFound,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
			}, fmt.Errorf("entity not found: %s", dbErr.Message)
		default:
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    defaultHeaders,
				Body:       `{"error":"Internal server error"}`,
			}, nil
		}
	}

	return &Response{
		StatusCode: http.StatusAccepted,
		Headers:    defaultHeaders,
		Body:       fmt.Sprintf(`{"message":"package validated successfully","package_id":"%s","supplier":"%s","session_id":"%s"}`, pkg.ID, pkg.Supplier.Name, sessionID),
	}, nil
}

type QualityCheckHandler struct {
	Passed bool     `json:"passed"`
	Issues []string `json:"issues"`
}

func (sr *ServiceRegistry) QualityCheckHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}
	sessionID := pathParts[2]

	var body QualityCheckHandler
	err := json.Unmarshal([]byte(req.Body), &body)
	if err != nil {
		sr.logger.Info("Failed to parse body", "error", err.Error())
		return &Response{
			StatusCode: http.StatusUnprocessableEntity,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid body format: %s"}`, err.Error()),
		}, fmt.Errorf("invalid body format")
	}

	pkg, qcRecord, dbErr := sr.repository.QualityCheck(sessionID, body.Passed, body.Issues)
	if dbErr != nil {
		switch dbErr.Code {
		case "ENTITY_NOT_FOUND":
			return &Response{
				StatusCode: http.StatusNotFound,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
			}, fmt.Errorf("entity not found: %s", dbErr.Message)
		default:
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    defaultHeaders,
				Body:       `{"error":"Internal server error"}`,
			}, nil
		}
	}
	if qcRecord == nil {
		return &Response{
			StatusCode: http.StatusNotFound,
			Headers:    defaultHeaders,
			Body:       `{"error":"qcRecord is nil"}`,
		}, err
	}

	return &Response{
		StatusCode: http.StatusAccepted,
		Headers:    defaultHeaders,
		Body: fmt.Sprintf(`{
		"message":"QC record created for package %s",
		"package_id":"%s",
		"qc_record_id":"%s",
		"operator_id": "%s"
		}`, pkg.ID, pkg.ID, qcRecord.ID, qcRecord.InspectorID),
	}, nil
}

type labelPackageHandlerBody struct {
	Label       string `json:"label"`
	Destination string `json:"destination"`
	Priority    string `json:"priority"`
	CourierID   string `json:"courier_id"`
}

// LabelPackageHandler assigns a label, courier, and destination
func (sr *ServiceRegistry) LabelPackageHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}
	sessionID := pathParts[2]

	var body labelPackageHandlerBody
	err := json.Unmarshal([]byte(req.Body), &body)
	if err != nil {
		return &Response{
			StatusCode: http.StatusUnprocessableEntity,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, err.Error()),
		}, err
	}

	newLabel, dbErr := sr.repository.LabelPackage(sessionID, body.Label, body.Destination, body.Priority, body.CourierID)
	if dbErr != nil {
		if dbErr.Code == "ENTITY_NOT_FOUND" {
			return &Response{
				StatusCode: http.StatusNotFound,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("database error: %v", dbErr)
		}
		if dbErr.Code == repository.PgErrForeignKeyViolation {
			return &Response{
				StatusCode: http.StatusBadRequest,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("database error: %v", dbErr)
		}
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
		}, fmt.Errorf("database error: %v", dbErr)
	}

	responseBody := fmt.Sprintf(`{"label_id":"%s"}`, newLabel.ID)
	return &Response{
		StatusCode: http.StatusAccepted,
		Headers:    defaultHeaders,
		Body:       responseBody,
	}, nil
}

type commitSessionHandlerBody struct {
	OperatorID string `json:"operator_id"`
}

// CommitSessionHandler commits the session to the chain
func (sr *ServiceRegistry) CommitSessionHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}
	sessionID := pathParts[2]

	var body commitSessionHandlerBody
	err := json.Unmarshal([]byte(req.Body), &body)
	if err != nil {
		return &Response{
			StatusCode: http.StatusUnprocessableEntity,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, err.Error()),
		}, err
	}

	tx, dbErr := sr.repository.CommitSession(sessionID, body.OperatorID)
	if dbErr != nil {
		if dbErr.Code == "INVALID_STATE" {
			return &Response{
				StatusCode: http.StatusConflict,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("database error: %v", dbErr)
		}
		if dbErr.Code == "ENTITY_NOT_FOUND" {
			return &Response{
				StatusCode: http.StatusNotFound,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("database error: %v", dbErr)
		}
		if dbErr.Code == repository.PgErrForeignKeyViolation {
			return &Response{
				StatusCode: http.StatusBadRequest,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("database error: %v", dbErr)
		}
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
		}, fmt.Errorf("database error: %v", dbErr)
	}

	txJsonBytes, _ := json.Marshal(tx)
	return &Response{
		StatusCode: http.StatusAccepted,
		Headers:    defaultHeaders,
		Body:       fmt.Sprintf(`{"tx":%s}`, txJsonBytes),
	}, nil
}

type receiveCommitHandlerBody struct {
	OperatorID        string    `json:"operator_id"`
	PackageID         string    `json:"package_id"`
	SupplierSignature string    `json:"supplier_signature"`
	QcPassed          bool      `json:"qc_passed"`
	Issues            []string  `json:"issues"`
	Timestamp         time.Time `json:"timestamp"`
	Label             string    `json:"label"`
	Destination       string    `json:"destination"`
	Priority          string    `json:"priority"`
	CourierID         string    `json:"courier_id"`
}

func (sr *ServiceRegistry) ReceiveCommitHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}
	sessionID := pathParts[2]

	var body receiveCommitHandlerBody
	err := json.Unmarshal([]byte(req.Body), &body)
	if err != nil {
		return &Response{
			StatusCode: http.StatusUnprocessableEntity,
			Headers:    defaultHeaders,
		}, err
	}

	repoErr := sr.repository.ReplicateCommitFromL2(sessionID, body.OperatorID, body.PackageID, body.SupplierSignature, body.Label, body.Destination, body.Priority, body.CourierID, body.QcPassed, body.Issues)
	if repoErr != nil {
		return &Response{
			StatusCode: http.StatusUnprocessableEntity,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s, %s, %s"}`, repoErr.Code, repoErr.Message, repoErr.Detail),
		}, err
	}

	return &Response{
		StatusCode: http.StatusAccepted,
		Headers:    defaultHeaders,
		Body:       req.Body,
	}, nil
}
