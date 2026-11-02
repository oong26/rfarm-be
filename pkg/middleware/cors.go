package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/oong26/rfarm-be/pkg/response"
)

const (
	corsAllowMethods  = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowHeaders  = "Authorization, Content-Type, If-Match, X-Request-ID"
	corsExposeHeaders = "ETag, X-Request-ID"
	corsMaxAge        = 600
)

// CORS allows the configured frontend origins only. Origins are matched
// exactly; "*" is supported but credentials are never enabled.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	wildcard := false
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		switch origin {
		case "":
			continue
		case "*":
			wildcard = true
		default:
			allowed[origin] = struct{}{}
		}
	}

	originAllowed := func(origin string) bool {
		if wildcard {
			return true
		}
		_, ok := allowed[origin]
		return ok
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Add("Vary", "Origin")

			if !originAllowed(origin) {
				if isPreflight(r) {
					response.Fail(w, r, response.Forbidden("Origin tidak diizinkan."))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			allowOrigin := origin
			if wildcard {
				allowOrigin = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Expose-Headers", corsExposeHeaders)

			if !isPreflight(r) {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(corsMaxAge))
			w.WriteHeader(http.StatusNoContent)
		})
	}
}

func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}
