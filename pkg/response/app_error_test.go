package response_test

import (
	"errors"
	"testing"

	"github.com/oong26/rfarm-be/pkg/response"
)

func TestAppError_Error(t *testing.T) {
	err := response.Conflict("data sudah ada")

	if got, want := err.Error(), "conflict: data sudah ada"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("database down")
	err := response.Internal(cause)

	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false, want true")
	}
}
