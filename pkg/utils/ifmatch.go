package utils

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/oong26/rfarm-be/pkg/response"
)

// ParseIfMatch reads the If-Match header and returns the row version. The
// header carries a quoted integer: If-Match: "4". Missing → 0 (caller
// treats as absent and calls PreconditionRequired). Malformed or wildcard
// → BadRequest.
func ParseIfMatch(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return 0, nil
	}

	// RFC 9110 specifies ETags may be quoted ("4"). Strip quotes.
	raw = strings.Trim(raw, "\"")

	// RFC 9110 allows a list of entity-tags; we accept only a single tag.
	// "*" means "any existing resource" and is not a valid version here.
	if raw == "*" {
		return 0, response.BadRequest(
			"Header If-Match harus berisi versi (angka), bukan '*'.",
		)
	}

	version, err := strconv.Atoi(raw)
	if err != nil {
		return 0, response.BadRequest(
			fmt.Sprintf("Header If-Match tidak valid: %s", raw),
		)
	}
	if version < 0 {
		return 0, response.BadRequest(
			"Header If-Match tidak boleh negatif.",
		)
	}
	return version, nil
}

// RequireIfMatch reads the If-Match header and fails the request when the
// header is absent or the version is 0. It returns the version otherwise.
func RequireIfMatch(r *http.Request) (int, error) {
	version, err := ParseIfMatch(r)
	if err != nil {
		return 0, err
	}
	if version == 0 {
		return 0, response.PreconditionRequired()
	}
	return version, nil
}
