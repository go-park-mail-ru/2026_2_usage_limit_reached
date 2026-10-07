package response_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
)

func TestDecodeJSON(t *testing.T) {
	type Payload struct {
		Name string `json:"name"`
	}

	t.Run("Success decode", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"name":"john"}`))
		req := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var payload Payload
		err := response.DecodeJSON(w, req, &payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payload.Name != "john" {
			t.Errorf("expected 'john', got %s", payload.Name)
		}
	})

	t.Run("Invalid JSON error", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{invalid-json`))
		req := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var payload Payload
		err := response.DecodeJSON(w, req, &payload)
		if err == nil {
			t.Fatalf("expected error on invalid JSON, got nil")
		}
	})
}

func TestWriteJSON(t *testing.T) {
	t.Run("Valid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := map[string]string{"message": "success"}

		response.WriteJSON(w, http.StatusAccepted, payload)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected status %d, got %d", http.StatusAccepted, w.Code)
		}
		if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		var res map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse json: %v", err)
		}
		if res["message"] != "success" {
			t.Errorf("expected body message 'success', got %s", res["message"])
		}
	})

	t.Run("Unmarshalable payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		unmarshalable := make(chan int)

		response.WriteJSON(w, http.StatusOK, unmarshalable)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500 on marshal error, got %d", w.Code)
		}
	})
}

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name           string
		invoke         func(w http.ResponseWriter)
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "ErrorInternal",
			invoke:         response.ErrorInternal,
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "internal server error",
		},
		{
			name:           "ErrorBadRequest",
			invoke:         response.ErrorBadRequest,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "bad request",
		},
		{
			name:           "ErrorUnauthorized",
			invoke:         response.ErrorUnauthorized,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "unauthorized",
		},
		{
			name:           "ErrorUserConflict",
			invoke:         response.ErrorUserConflict,
			expectedStatus: http.StatusConflict,
			expectedError:  "user already exist",
		},
		{
			name:           "ErrorNotFound",
			invoke:         response.ErrorNotFound,
			expectedStatus: http.StatusNotFound,
			expectedError:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.invoke(w)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}

			var resp response.ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Error != tt.expectedError {
				t.Errorf("expected error message %q, got %q", tt.expectedError, resp.Error)
			}
		})
	}
}