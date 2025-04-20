package srvreg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository"
)

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
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       fmt.Sprintf(`{"error":"Failed to create session: %s"}`, err.Error()),
		}, err
	}

	operatorID := body.OperatorID
	if operatorID == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"error":"operator ID is required"}`,
		}, err
	}

	session, dbErr := sr.repository.CreateSession(sessionID, operatorID)
	if dbErr != nil {
		switch dbErr.Code {
		case repository.PgErrForeignKeyViolation: // PostgreSQL foreign key violation
			return &Response{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("foreign key violation: %s", dbErr.Message)
		case repository.PgErrUniqueViolation: // PostgreSQL unique violation
			return &Response{
				StatusCode: http.StatusConflict,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Detail),
			}, fmt.Errorf("unique violation: %s", dbErr.Message)
		default:
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error":"Internal server error"}`,
			}, nil
		}
	}

	return &Response{
		StatusCode: http.StatusCreated, // or http.StatusOK
		Headers:    map[string]string{"Content-Type": "application/json"},
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
		return nil, err
	}

	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"Content-Type": "application/json"},
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
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
			}, fmt.Errorf("entity not found: %s", dbErr.Message)
		default:
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
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
			Headers:    map[string]string{"Content-Type": "application/json"},
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
		Headers:    map[string]string{"Content-Type": "application/json"},
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
		return nil, err
	}
	if body.Signature == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"error":"signature is required"}`,
		}, err
	}
	if body.PackageID == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"error":"package_id is required"}`,
		}, err
	}

	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"Content-Type": "application/json"},
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
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
			}, fmt.Errorf("entity not found: %s", dbErr.Message)
		default:
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error":"Internal server error"}`,
			}, nil
		}
	}

	return &Response{
		StatusCode: http.StatusAccepted,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"message":"package validated successfully","package_id":"%s","supplier":"%s","session_id":"%s"}`, pkg.ID, pkg.Supplier.Name, sessionID),
	}, nil
}
