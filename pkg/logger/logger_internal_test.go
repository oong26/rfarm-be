package logger

import (
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  slog.Level
	}{
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "info", input: "info", want: slog.LevelInfo},
		{name: "warn", input: "warn", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
		{name: "case and spaces", input: "  WARN  ", want: slog.LevelWarn},
		{name: "empty", input: "", want: slog.LevelInfo},
		{name: "unknown", input: "verbose", want: slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLevel(tt.input); got != tt.want {
				t.Errorf("parseLevel(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}
