package middleware_test

import (
	"context"
	"testing"

	"github.com/oong26/rfarm-be/pkg/middleware"
)

func TestPrincipal(t *testing.T) {
	ctx := middleware.WithPrincipal(context.Background(), middleware.Principal{
		UserID: "user-1",
		FarmID: "farm-1",
		Role:   "owner",
	})

	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok {
		t.Fatal("PrincipalFrom() = not found, want the stored principal")
	}
	if principal.UserID != "user-1" || principal.FarmID != "farm-1" || principal.Role != "owner" {
		t.Errorf("principal = %+v, want user-1/farm-1/owner", principal)
	}
	if got := middleware.FarmIDFrom(ctx); got != "farm-1" {
		t.Errorf("FarmIDFrom() = %q, want farm-1", got)
	}
	if got := middleware.UserIDFrom(ctx); got != "user-1" {
		t.Errorf("UserIDFrom() = %q, want user-1", got)
	}
}

func TestPrincipal_Missing(t *testing.T) {
	ctx := context.Background()

	if _, ok := middleware.PrincipalFrom(ctx); ok {
		t.Error("PrincipalFrom() found a principal in an empty context")
	}
	if got := middleware.FarmIDFrom(ctx); got != "" {
		t.Errorf("FarmIDFrom() = %q, want empty", got)
	}
	if got := middleware.UserIDFrom(ctx); got != "" {
		t.Errorf("UserIDFrom() = %q, want empty", got)
	}
}
