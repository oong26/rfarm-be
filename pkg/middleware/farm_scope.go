package middleware

import (
	"net/http"

	"github.com/oong26/rfarm-be/pkg/response"
)

// FarmScope rejects a request that reached a farm route without an
// authenticated farm. The farm always comes from the token, never from a
// header, query parameter or body.
func FarmScope() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFrom(r.Context())
			if !ok || principal.FarmID == "" {
				response.Fail(w, r, response.Unauthorized("Sesi tidak valid. Silakan masuk kembali."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
