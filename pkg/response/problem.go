package response

import (
	"encoding/json"
	"errors"
	"net/http"
)

const problemBaseURL = "https://rfarm.example/problems/"

// Problem is the wire form documented in docs/error_handling.md.
type Problem struct {
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Status    int     `json:"status"`
	Code      string  `json:"code"`
	Detail    string  `json:"detail,omitempty"`
	RequestID string  `json:"requestId"`
	Errors    []Field `json:"errors,omitempty"`
}

// Fail writes err as an RFC 9457 problem body. Unknown errors become a generic
// 500; the cause never reaches the client.
func Fail(w http.ResponseWriter, _ *http.Request, err error) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		appErr = Internal(err)
	}
	writeProblem(w, appErr)
}

func writeProblem(w http.ResponseWriter, appErr *AppError) {
	body, err := json.Marshal(Problem{
		Type:      problemBaseURL + appErr.Code,
		Title:     appErr.Title,
		Status:    appErr.Status,
		Code:      appErr.Code,
		Detail:    appErr.Message,
		RequestID: w.Header().Get("X-Request-ID"),
		Errors:    appErr.Fields,
	})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(appErr.Status)
	_, _ = w.Write(body)
}
