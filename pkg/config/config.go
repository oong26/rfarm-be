// Package config loads a YAML file into a typed configuration struct.
//
// The file named by CONFIG_PATH (default ./config.yaml) is read once at startup.
// Precedence, highest last, is: defaults in code, the YAML file, environment
// overrides declared with `env` struct tags.
//
// The package knows nothing about the application's sections; the struct it
// fills (and that struct's defaults and validation) belongs to the caller.
//
//	type Config struct {
//		Database struct {
//			URL string `yaml:"url" env:"DATABASE_URL"`
//		} `yaml:"database"`
//	}
//
//	cfg, err := config.Load[Config]()
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"

	"gopkg.in/yaml.v3"
)

const (
	// PathEnv names the environment variable that selects the configuration file.
	PathEnv = "CONFIG_PATH"
	// DefaultPath is used when PathEnv is unset.
	DefaultPath = "./config.yaml"
)

// Defaulter is implemented by configuration structs that carry defaults in code.
// It runs before the file is decoded, so keys present in the file win.
type Defaulter interface {
	SetDefaults()
}

// Validator is implemented by configuration structs that must be checked after
// the file and the environment have been applied. Errors should name the key.
type Validator interface {
	Validate() error
}

// Option customises a Load call.
type Option func(*options)

type options struct {
	path string
}

// WithPath overrides the file path for one Load call.
func WithPath(path string) Option {
	return func(o *options) {
		o.path = path
	}
}

// Path returns the configuration file path: CONFIG_PATH when set, otherwise
// DefaultPath.
func Path() string {
	if path := os.Getenv(PathEnv); path != "" {
		return path
	}
	return DefaultPath
}

// Load reads, decodes, overrides and validates a configuration file into T.
//
// T must be a struct type. Load applies T.SetDefaults (if implemented) before
// decoding, environment overrides after decoding, and T.Validate (if
// implemented) last. Empty files are allowed and simply keep the defaults.
// An unset or empty environment variable keeps the value from the file.
func Load[T any](opts ...Option) (*T, error) {
	o := options{path: Path()}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.path == "" {
		return nil, errors.New("config: path is empty")
	}

	data, err := os.ReadFile(o.path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", o.path, err)
	}

	cfg := new(T)
	if err := checkStruct(cfg); err != nil {
		return nil, err
	}
	if d, ok := any(cfg).(Defaulter); ok {
		d.SetDefaults()
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("config: parse %s: %w", o.path, err)
	}

	if err := applyEnv(cfg); err != nil {
		return nil, fmt.Errorf("config: %s: %w", o.path, err)
	}

	if v, ok := any(cfg).(Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, fmt.Errorf("config: validate %s: %w", o.path, err)
		}
	}

	return cfg, nil
}

// checkStruct rejects pointer type parameters early, before decoding or
// reflection produce a confusing error.
func checkStruct(target any) error {
	t := reflect.TypeOf(target)
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("config: type parameter must be a struct type, got %s", t)
	}
	return nil
}
