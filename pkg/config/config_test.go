package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oong26/rfarm-be/pkg/config"
)

type appSection struct {
	Env     string          `yaml:"env"`
	Timeout config.Duration `yaml:"timeout"`
}

type databaseSection struct {
	URL string `yaml:"url" env:"TEST_DATABASE_URL"`
}

type testConfig struct {
	App      appSection      `yaml:"app"`
	Database databaseSection `yaml:"database"`
}

func (c *testConfig) SetDefaults() {
	if c.App.Env == "" {
		c.App.Env = "development"
	}
	if c.App.Timeout == 0 {
		c.App.Timeout = config.Duration(10 * time.Second)
	}
}

func (c *testConfig) Validate() error {
	if c.Database.URL == "" {
		return errors.New("database.url is required")
	}
	return nil
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoad(t *testing.T) {
	t.Run("defaults apply before the file", func(t *testing.T) {
		path := writeConfig(t, "database:\n  url: postgres://file\n")

		got, err := config.Load[testConfig](config.WithPath(path))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got.App.Env != "development" {
			t.Errorf("app.env = %q, want development", got.App.Env)
		}
		if got.App.Timeout.Duration() != 10*time.Second {
			t.Errorf("app.timeout = %s, want 10s", got.App.Timeout)
		}
	})

	t.Run("file overrides defaults", func(t *testing.T) {
		path := writeConfig(t, `
app:
  env: staging
  timeout: 30s
database:
  url: postgres://file
`)

		got, err := config.Load[testConfig](config.WithPath(path))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got.App.Env != "staging" {
			t.Errorf("app.env = %q, want staging", got.App.Env)
		}
		if got.App.Timeout.Duration() != 30*time.Second {
			t.Errorf("app.timeout = %s, want 30s", got.App.Timeout)
		}
	})

	t.Run("environment overrides the file", func(t *testing.T) {
		t.Setenv("TEST_DATABASE_URL", "postgres://env")
		path := writeConfig(t, "database:\n  url: postgres://file\n")

		got, err := config.Load[testConfig](config.WithPath(path))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got.Database.URL != "postgres://env" {
			t.Errorf("database.url = %q, want postgres://env", got.Database.URL)
		}
	})

	t.Run("empty environment variable keeps the file value", func(t *testing.T) {
		t.Setenv("TEST_DATABASE_URL", "")
		path := writeConfig(t, "database:\n  url: postgres://file\n")

		got, err := config.Load[testConfig](config.WithPath(path))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got.Database.URL != "postgres://file" {
			t.Errorf("database.url = %q, want postgres://file", got.Database.URL)
		}
	})

	t.Run("empty file keeps defaults", func(t *testing.T) {
		t.Setenv("TEST_DATABASE_URL", "postgres://env")
		path := writeConfig(t, "")

		got, err := config.Load[testConfig](config.WithPath(path))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got.App.Env != "development" {
			t.Errorf("app.env = %q, want development", got.App.Env)
		}
	})

	t.Run("required value fails validation", func(t *testing.T) {
		path := writeConfig(t, "app:\n  env: production\n")

		_, err := config.Load[testConfig](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "database.url is required") {
			t.Errorf("Load() error = %v, want it to name database.url", err)
		}
	})

	t.Run("unknown key is rejected", func(t *testing.T) {
		path := writeConfig(t, `
app:
  env: development
  typo: true
database:
  url: postgres://file
`)

		_, err := config.Load[testConfig](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want unknown key error")
		}
		if !strings.Contains(err.Error(), "field typo not found") {
			t.Errorf("Load() error = %v, want it to name the unknown key", err)
		}
	})

	t.Run("missing file names the path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "absent.yaml")

		_, err := config.Load[testConfig](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want read error")
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("Load() error = %v, want it to name %s", err, path)
		}
	})

	t.Run("CONFIG_PATH selects the file", func(t *testing.T) {
		path := writeConfig(t, "database:\n  url: postgres://file\n")
		t.Setenv(config.PathEnv, path)

		if _, err := config.Load[testConfig](); err != nil {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("WithPath overrides CONFIG_PATH", func(t *testing.T) {
		path := writeConfig(t, "database:\n  url: postgres://file\n")
		t.Setenv(config.PathEnv, filepath.Join(t.TempDir(), "absent.yaml"))

		if _, err := config.Load[testConfig](config.WithPath(path)); err != nil {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("pointer type parameter is rejected", func(t *testing.T) {
		path := writeConfig(t, "database:\n  url: postgres://file\n")

		_, err := config.Load[*testConfig](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want type error")
		}
		if !strings.Contains(err.Error(), "struct type") {
			t.Errorf("Load() error = %v, want it to require a struct type", err)
		}
	})
}

func TestPath(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv(config.PathEnv, "")

		if got := config.Path(); got != config.DefaultPath {
			t.Errorf("Path() = %q, want %q", got, config.DefaultPath)
		}
	})

	t.Run("from environment", func(t *testing.T) {
		t.Setenv(config.PathEnv, "/etc/rfarm/config.yaml")

		if got := config.Path(); got != "/etc/rfarm/config.yaml" {
			t.Errorf("Path() = %q, want /etc/rfarm/config.yaml", got)
		}
	})
}
