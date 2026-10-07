package response_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err, "unexpected error: %v", err)
		require.Equal(t, "john", payload.Name, "expected 'john', got %s", payload.Name)
	})

	t.Run("Invalid JSON error", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{invalid-json`))
		req := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var payload Payload
		err := response.DecodeJSON(w, req, &payload)
		require.Error(t, err)
	})
}

func TestWriteJSON(t *testing.T) {
	t.Run("Valid payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := map[string]string{"message": "success"}

		response.WriteJSON(w, http.StatusAccepted, payload)

		require.Equal(t, http.StatusAccepted, w.Code)

		contentType := w.Header().Get("Content-Type")
		require.Equal(t, "application/json", contentType)

		var res map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)
		require.Equal(t, "success", res["message"], "expected body message 'success', got %s", res["message"])
	})

	t.Run("Unmarshalable payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		unmarshalable := make(chan int)

		response.WriteJSON(w, http.StatusOK, unmarshalable)

		require.Equal(t, http.StatusInternalServerError, w.Code)
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

			require.Equal(t, tt.expectedStatus, w.Code)

			contentType := w.Header().Get("Content-Type")
			require.Equal(t, "application/json", contentType)

			var resp response.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			require.Equal(t, tt.expectedError, resp.Error, "expected error message %q, got %q", tt.expectedError, resp.Error)
		})
	}
}
