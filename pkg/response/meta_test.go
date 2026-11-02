package response_test

import (
	"testing"

	"github.com/oong26/rfarm-be/pkg/response"
)

func TestNewMeta(t *testing.T) {
	got := response.NewMeta(3, 50, 101)

	if got.Page != 3 {
		t.Errorf("Page = %d, want 3", got.Page)
	}
	if got.PageSize != 50 {
		t.Errorf("PageSize = %d, want 50", got.PageSize)
	}
	if got.Total != 101 {
		t.Errorf("Total = %d, want 101", got.Total)
	}
}
