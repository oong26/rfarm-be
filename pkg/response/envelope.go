package response

import (
	"encoding/json"
	"net/http"
)

// Envelope is the success body: data for every response, meta only on lists.
type Envelope struct {
	Data any   `json:"data"`
	Meta *Meta `json:"meta,omitempty"`
}

// OK writes a 200 response with data in the envelope.
func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Envelope{Data: data})
}

// Created writes a 201 response with the created resource in the envelope.
func Created(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, Envelope{Data: data})
}

// NoContent writes a 204 response with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// List writes a paginated 200 response. A nil slice is encoded as [] so the
// wire form is always an array.
func List[T any](w http.ResponseWriter, data []T, meta Meta) {
	if data == nil {
		data = make([]T, 0)
	}
	writeJSON(w, http.StatusOK, Envelope{Data: data, Meta: &meta})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
