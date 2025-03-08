package validator

import (
	"fmt"
	"github.com/go-playground/validator/v10"
)

// MessageFunc is a function that generates a custom error message for a validation error
type MessageFunc func(field string, param string) string

// DefaultMessageMap contains the default error message functions for built-in validation tags
var DefaultMessageMap = map[string]MessageFunc{
	"required": func(field, _ string) string {
		return fmt.Sprintf("%s is required", field)
	},
	"email": func(field, _ string) string {
		return fmt.Sprintf("%s must be a valid email address", field)
	},
	"min": func(field, param string) string {
		return fmt.Sprintf("%s must be at least %s", field, param)
	},
	"max": func(field, param string) string {
		return fmt.Sprintf("%s must not exceed %s", field, param)
	},
	"len": func(field, param string) string {
		return fmt.Sprintf("%s must be exactly %s characters long", field, param)
	},
	// Default fallback for any other validation tags
	"default": func(field, tag string) string {
		return fmt.Sprintf("%s failed validation for rule: %s", field, tag)
	},
}

// createUserFriendlyMessage uses the custom message functions to generate error messages
func (v *Validator) createUserFriendlyMessage(field, tag string, err validator.FieldError) string {
	// Look for a custom message function for this tag
	if messageFn, exists := v.messages[tag]; exists {
		return messageFn(field, err.Param())
	}

	// Fall back to a default message if available
	if defaultFn, exists := v.messages["default"]; exists {
		return defaultFn(field, tag)
	}

	// Last resort fallback
	return fmt.Sprintf("%s failed validation for rule: %s", field, tag)
}
