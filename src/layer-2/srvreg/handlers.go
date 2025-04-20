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

type scanHandlerBody struct {
	PackageID string `json:"package_id"`
}

func (sr *ServiceRegistry) ScanHandler(req *Request) (*Response, error) {
	var body scanHandlerBody
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

	pkg, dbErr := sr.repository.ValidatePackage(sessionID, body.PackageID)
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
		StatusCode: http.StatusOK,
		Body:       fmt.Sprintf(`{"message":"Package validated","package_id":"%s",session_id:"%s"}`, pkg.ID, sessionID),
	}, nil
}
