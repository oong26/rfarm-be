package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// AccessLog records one structured line per request: status, size, latency and
// the identity known when the response finished.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := &requestState{}
			if principal, ok := PrincipalFrom(r.Context()); ok {
				state.set(principal)
			}
			ctx := context.WithValue(r.Context(), requestStateKey{}, state)
			r = r.WithContext(ctx)

			recorder := &statusRecorder{ResponseWriter: w}
			start := time.Now()

			next.ServeHTTP(recorder, r)

			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}

			fields := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", recorder.bytes,
				"latency_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFrom(ctx),
			}
			if userID, farmID := state.identity(); userID != "" || farmID != "" {
				if userID != "" {
					fields = append(fields, "user_id", userID)
				}
				if farmID != "" {
					fields = append(fields, "farm_id", farmID)
				}
			}

			switch {
			case status >= http.StatusInternalServerError:
				log.Error("http request", fields...)
			case status >= http.StatusBadRequest:
				log.Warn("http request", fields...)
			default:
				log.Info("http request", fields...)
			}
		})
	}
}

// statusRecorder captures the status and byte count without hiding the original
// writer, so http.ResponseController keeps working through Unwrap.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += n
	return n, err
}

func (w *statusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
