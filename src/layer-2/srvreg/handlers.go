package srvreg

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ahmadzakiakmal/thesis/src/layer-2/repository"
)

type startSessionBody struct {
	OperatorID string `json:"operator_id"`
}

func (sr *ServiceRegistry) CreateSessionHandler(req *Request) (*Response, error) {
	sessionID := fmt.Sprintf("SESSION-%s", req.RequestID)
	var body startSessionBody
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
		fmt.Println("PG ERROR CODE")
		fmt.Println(dbErr.Code)
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
