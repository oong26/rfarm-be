package config

import (
	"fmt"
	"os"
	"reflect"
)

// envTag names the struct tag that maps a field to an environment variable:
//
//	type Database struct {
//		URL string `yaml:"url" env:"DATABASE_URL"`
//	}
//
// Only string fields are supported: the values that may leave the file are
// secrets such as a database URL or a signing key. An unset or empty variable
// keeps the value from the file.
const envTag = "env"

// applyEnv walks the configuration struct and overwrites tagged fields from the
// environment. Invalid tags fail the load instead of being silently ignored.
func applyEnv(target any) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return fmt.Errorf("env: target must be a non-nil pointer, got %T", target)
	}
	return walkEnv(v.Elem())
}

func walkEnv(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		value := v.Field(i)
		switch value.Kind() {
		case reflect.Struct:
			if name := field.Tag.Get(envTag); name != "" {
				return fmt.Errorf("%s.%s: env %s overrides string fields only", t, field.Name, name)
			}
			if err := walkEnv(value); err != nil {
				return err
			}
		case reflect.String:
			name := field.Tag.Get(envTag)
			if name == "" {
				continue
			}
			if fromEnv, ok := os.LookupEnv(name); ok && fromEnv != "" {
				value.SetString(fromEnv)
			}
		default:
			if name := field.Tag.Get(envTag); name != "" {
				return fmt.Errorf("%s.%s: env %s overrides string fields only", t, field.Name, name)
			}
		}
	}
	return nil
}
