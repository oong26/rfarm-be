package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/oong26/rfarm-be/pkg/config"
)

type envOnlyConfig struct {
	Secret string `yaml:"secret" env:"TEST_ENV_ONLY_SECRET"`
}

func (c *envOnlyConfig) Validate() error {
	if c.Secret == "" {
		return errors.New("secret is required")
	}
	return nil
}

func TestLoad_Env(t *testing.T) {
	t.Run("applies before validation", func(t *testing.T) {
		t.Setenv("TEST_ENV_ONLY_SECRET", "from-env")
		path := writeConfig(t, "")

		got, err := config.Load[envOnlyConfig](config.WithPath(path))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.Secret != "from-env" {
			t.Errorf("secret = %q, want from-env", got.Secret)
		}
	})

	t.Run("missing value fails validation", func(t *testing.T) {
		t.Setenv("TEST_ENV_ONLY_SECRET", "")
		path := writeConfig(t, "")

		_, err := config.Load[envOnlyConfig](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "secret is required") {
			t.Errorf("Load() error = %v, want secret is required", err)
		}
	})

	t.Run("non-string field is rejected", func(t *testing.T) {
		type bad struct {
			Port int `yaml:"port" env:"TEST_BAD_PORT"`
		}
		path := writeConfig(t, "port: 8080\n")

		_, err := config.Load[bad](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want unsupported field error")
		}
		if !strings.Contains(err.Error(), "string fields only") {
			t.Errorf("Load() error = %v, want string fields only", err)
		}
	})

	t.Run("struct field is rejected", func(t *testing.T) {
		type inner struct {
			Name string `yaml:"name"`
		}
		type bad struct {
			Inner inner `yaml:"inner" env:"TEST_BAD_INNER"`
		}
		path := writeConfig(t, "inner:\n  name: value\n")

		_, err := config.Load[bad](config.WithPath(path))
		if err == nil {
			t.Fatal("Load() error = nil, want unsupported field error")
		}
		if !strings.Contains(err.Error(), "string fields only") {
			t.Errorf("Load() error = %v, want string fields only", err)
		}
	})
}
