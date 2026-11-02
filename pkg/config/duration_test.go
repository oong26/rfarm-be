package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/oong26/rfarm-be/pkg/config"
	"gopkg.in/yaml.v3"
)

func TestDuration(t *testing.T) {
	t.Run("decodes Go duration strings", func(t *testing.T) {
		path := writeConfig(t, `
app:
  timeout: 720h
database:
  url: postgres://file
`)

		got, err := config.Load[testConfig](config.WithPath(path))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.App.Timeout.Duration() != 720*time.Hour {
			t.Errorf("timeout = %s, want 720h", got.App.Timeout)
		}
	})

	t.Run("rejects a bare number", func(t *testing.T) {
		path := writeConfig(t, `
app:
  timeout: 10
database:
  url: postgres://file
`)

		_, err := config.Load[testConfig](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want invalid duration error")
		}
		if !strings.Contains(err.Error(), "duration must be a string") {
			t.Errorf("Load() error = %v, want duration must be a string", err)
		}
	})

	t.Run("rejects a unitless string", func(t *testing.T) {
		path := writeConfig(t, `
app:
  timeout: soon
database:
  url: postgres://file
`)

		_, err := config.Load[testConfig](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want invalid duration error")
		}
		if !strings.Contains(err.Error(), `invalid duration "soon"`) {
			t.Errorf("Load() error = %v, want it to quote the invalid value", err)
		}
	})

	t.Run("marshals as a duration string", func(t *testing.T) {
		body, err := yaml.Marshal(struct {
			Timeout config.Duration `yaml:"timeout"`
		}{Timeout: config.Duration(15 * time.Minute)})
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if got, want := strings.TrimSpace(string(body)), "timeout: 15m0s"; got != want {
			t.Errorf("Marshal() = %q, want %q", got, want)
		}
	})
}
