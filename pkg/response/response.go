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

func ErrorInternal(w http.ResponseWriter) {
	write(w, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
}

func ErrorBadRequest(w http.ResponseWriter) {
	write(w, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
}

func ErrorUnauthorized(w http.ResponseWriter) {
	write(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
}

func ErrorUserConflict(w http.ResponseWriter) {
	write(w, http.StatusConflict, ErrorResponse{Error: "user already exist"})
}

func ErrorNotFound(w http.ResponseWriter) {
	write(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
}

func write(w http.ResponseWriter, code int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		ErrorInternal(w)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(data)
}
