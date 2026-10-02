package response

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(dst)
}

func WriteJSON(w http.ResponseWriter, code int, payload any) {
	write(w, code, payload)
}

func Error(w http.ResponseWriter, code int, msg string) {
	write(w, code, ErrorResponse{Error: msg})
}

func write(w http.ResponseWriter, code int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal server error")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(data)
}
