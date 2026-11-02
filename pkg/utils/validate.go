// Package utils provides general-purpose helpers shared across every layer:
// validation wrappers, query parsing, If-Match header parsing, and
// date/decimal/clock utilities.
package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/oong26/rfarm-be/pkg/response"
)

var validate = validator.New()

func init() {
	// Use the `query` tag name so struct field names in error messages match
	// the wire name the client sent, not the Go field name.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := fld.Tag.Get("query")
		if name == "" {
			name = fld.Tag.Get("json")
		}
		if name == "" || name == "-" {
			return fld.Name
		}
		return strings.Split(name, ",")[0]
	})
}

// ParseAndValidate decodes the JSON body and validates the result struct.
// On a bad body it returns a BadRequest error; on field failures it returns
// an Unprocessable error with one Field per violation.
func ParseBody[T any](r *http.Request, dst *T) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return response.BadRequest("Body permintaan tidak valid: " + sanitizeJSONErr(err))
	}
	return validateStruct(dst)
}

// ParseAndValidate is the standard handler entry point: it decodes the JSON
// body into dst and runs struct-tag validation. Use ParseQuery separately for
// query parameters.
func ParseAndValidate(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return response.BadRequest("Body permintaan tidak valid: " + sanitizeJSONErr(err))
	}
	return validateStruct(dst)
}

// Validate runs struct-tag validation on an already-populated struct.
// Returns nil or an *response.AppError with field-level errors.
func Validate(dst any) error {
	return validateStruct(dst)
}

// validateStruct runs go-playground/validator and maps its errors to the
// response layer.
func validateStruct(dst any) error {
	err := validate.Struct(dst)
	if err == nil {
		return nil
	}

	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return response.BadRequest(err.Error())
	}

	fields := make([]response.Field, 0, len(ve))
	for _, fe := range ve {
		fields = append(fields, response.Field{
			Field:   fe.Field(),
			Code:    validatorTagToCode(fe.Tag()),
			Message: validationMessage(fe),
		})
	}
	return response.Unprocessable(fields...)
}

// validatorTagToCode maps go-playground tag names to the stable codes the
// frontend relies on (see spec §9.1).
func validatorTagToCode(tag string) string {
	switch tag {
	case "required":
		return "required"
	case "min", "gte":
		return "min"
	case "max", "lte":
		return "max"
	case "oneof":
		return "invalid_enum"
	case "datetime":
		return "invalid_format"
	case "uuid", "uuid4":
		return "invalid_uuid"
	case "email":
		return "invalid_email"
	default:
		return tag
	}
}

// validationMessage returns an Indonesian user-facing message for a field
// error. Generic fallback keeps the English tag name so nothing is silent.
func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("Field %s wajib diisi.", fe.Field())
	case "min", "gte":
		return fmt.Sprintf("Field %s minimal %s.", fe.Field(), fe.Param())
	case "max", "lte":
		return fmt.Sprintf("Field %s maksimal %s.", fe.Field(), fe.Param())
	case "oneof":
		return fmt.Sprintf("Field %s harus salah satu dari: %s.", fe.Field(), fe.Param())
	case "datetime":
		return fmt.Sprintf("Field %s harus berformat %s.", fe.Field(), fe.Param())
	case "email":
		return fmt.Sprintf("Field %s harus berupa alamat email yang valid.", fe.Field())
	default:
		return fmt.Sprintf("Field %s tidak valid (%s).", fe.Field(), fe.Tag())
	}
}

// sanitizeJSONErr strips file offsets that leak internal details.
func sanitizeJSONErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// json.UnmarshalTypeError and json.SyntaxError both contain a byte offset.
	// Keep the message but not the byte count so it stays readable.
	if idx := strings.Index(msg, " at offset"); idx != -1 {
		msg = msg[:idx]
	}
	return msg
}

// QueryInt reads a query parameter as int, returning def when absent.
// Returns an InvalidQuery error when the value is present but not an integer.
func QueryInt(r *http.Request, key string, def int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, response.InvalidQuery(
			fmt.Sprintf("Parameter '%s' harus berupa bilangan bulat.", key),
		)
	}
	return v, nil
}

// QueryString reads a query parameter as a trimmed string, returning def when
// absent or blank.
func QueryString(r *http.Request, key, def string) string {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	return v
}
