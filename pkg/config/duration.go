package config

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that decodes the Go duration strings used in the
// YAML file ("10s", "15m", "720h") instead of raw nanoseconds.
type Duration time.Duration

// Duration returns the standard library value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// String renders the value as a Go duration string.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// UnmarshalYAML accepts only string scalars so a bare number is reported with a
// readable message instead of being read as nanoseconds.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("duration must be a string such as %q, got %q", "10s", node.Value)
	}

	value := strings.TrimSpace(node.Value)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value, err)
	}

	*d = Duration(parsed)
	return nil
}

// MarshalYAML keeps a round-trip readable as a duration string.
func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}
