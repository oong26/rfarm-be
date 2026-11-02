package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/oong26/rfarm-be/pkg/logger"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantDebug bool
	}{
		{name: "debug keeps debug", level: "debug", wantDebug: true},
		{name: "info drops debug", level: "info", wantDebug: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := logger.New(logger.Options{Level: tt.level, Writer: &buf})

			log.Debug("debug-line")
			log.Info("info-line")
			out := buf.String()

			if got := strings.Contains(out, "debug-line"); got != tt.wantDebug {
				t.Errorf("debug present = %v, want %v; output: %s", got, tt.wantDebug, out)
			}
			if !strings.Contains(out, "info-line") {
				t.Errorf("info line missing; output: %s", out)
			}
		})
	}
}

func TestNew_JSON(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Options{JSON: true, Writer: &buf})

	log.Info("started", "addr", ":8080")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not JSON: %v; output: %s", err, buf.String())
	}
	if record["msg"] != "started" {
		t.Errorf("msg = %v, want started", record["msg"])
	}
	if record["addr"] != ":8080" {
		t.Errorf("addr = %v, want :8080", record["addr"])
	}
}

func TestHelpers(t *testing.T) {
	tests := []struct {
		name      string
		log       func(string, ...any)
		wantLevel string
	}{
		{name: "debug", log: logger.Debug, wantLevel: "DEBUG"},
		{name: "info", log: logger.Info, wantLevel: "INFO"},
		{name: "warn", log: logger.Warn, wantLevel: "WARN"},
		{name: "error", log: logger.Error, wantLevel: "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(logger.New(logger.Options{Level: "debug", Writer: &buf}))
			t.Cleanup(func() { slog.SetDefault(previous) })

			tt.log("hello", "addr", ":8080")
			out := buf.String()

			if !strings.Contains(out, "level="+tt.wantLevel) {
				t.Errorf("level = %s missing; output: %s", tt.wantLevel, out)
			}
			if !strings.Contains(out, "addr=:8080") {
				t.Errorf("key/value pair missing; output: %s", out)
			}
		})
	}
}
