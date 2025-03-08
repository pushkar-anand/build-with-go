package validator

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestValidatorCreateUserFriendlyMessage_defaultMessages tests the default error messages
func TestValidatorCreateUserFriendlyMessage_defaultMessages(t *testing.T) {
	resetValidator()

	// Create validator with default messages
	v, err := New()
	require.NoError(t, err)
	require.NotNil(t, v)

	// Define test cases for default messages
	tests := []struct {
		name        string
		input       TestStruct
		field       string
		expectedMsg string
	}{
		{
			name: "Required field",
			input: TestStruct{
				// Missing ID - required field
				Name:     "John",
				Email:    "john@example.com",
				Age:      30,
				Password: "password123",
			},
			field:       "id",
			expectedMsg: "id is required",
		},
		{
			name: "Min length field",
			input: TestStruct{
				ID:       1,
				Name:     "Jo", // Too short
				Email:    "john@example.com",
				Age:      30,
				Password: "password123",
			},
			field:       "name",
			expectedMsg: "name must be at least 3",
		},
		{
			name: "Email field",
			input: TestStruct{
				ID:       1,
				Name:     "John",
				Email:    "not-an-email", // Invalid email
				Age:      30,
				Password: "password123",
			},
			field:       "email",
			expectedMsg: "email must be a valid email address",
		},
		{
			name: "Min value field",
			input: TestStruct{
				ID:       1,
				Name:     "John",
				Email:    "john@example.com",
				Age:      16, // Too young
				Password: "password123",
			},
			field:       "age",
			expectedMsg: "age must be at least 18",
		},
		{
			name: "Max value field",
			input: TestStruct{
				ID:       1,
				Name:     "John",
				Email:    "john@example.com",
				Age:      130, // Too old
				Password: "password123",
			},
			field:       "age",
			expectedMsg: "age must not exceed 120",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateStruct(context.Background(), tt.input)
			require.NoError(t, err)
			assert.False(t, result.Valid)

			// Check that the specific field has the expected error message
			reason, exists := result.Failed[tt.field]
			assert.True(t, exists, "Field %s should have an error", tt.field)
			assert.Equal(t, tt.expectedMsg, reason.Message)
		})
	}
}

var customMessages = map[string]MessageFunc{
	"min": func(field, param string) string {
		if field == "age" {
			return fmt.Sprintf("You must be at least %s years old", param)
		}
		return fmt.Sprintf("The %s must have at least %s characters", field, param)
	},
	"max": func(field, param string) string {
		if field == "age" {
			return fmt.Sprintf("You cannot be older than %s years", param)
		}
		return fmt.Sprintf("The %s cannot exceed %s", field, param)
	},
	// Default fallback message
	"default": func(field, tag string) string {
		return fmt.Sprintf("Custom validation failed: %s did not pass %s", field, tag)
	},
}

// TestValidatorCreateUserFriendlyMessage_customMessages tests all custom error messages with a single validator
func TestValidatorCreateUserFriendlyMessage_customMessages(t *testing.T) {
	resetValidator()

	// Create a single validator with all custom messages
	v, err := New(
		// Individual custom messages
		WithCustomMessage("required", func(field, _ string) string {
			return fmt.Sprintf("Please provide a value for %s", field)
		}),
		WithCustomMessage("email", func(field, _ string) string {
			return "Please enter a valid email address"
		}),
		// Bulk custom messages
		WithCustomMessages(customMessages),
	)
	require.NoError(t, err)
	require.NotNil(t, v)

	// Define test cases for custom messages
	tests := []struct {
		name        string
		input       TestStruct
		field       string
		expectedMsg string
	}{
		{
			name:        "Required field custom message",
			input:       TestStruct{}, // Missing all required fields
			field:       "id",
			expectedMsg: "Please provide a value for id",
		},
		{
			name: "Min length field custom message",
			input: TestStruct{
				ID:       1,
				Name:     "Jo", // Too short
				Email:    "john@example.com",
				Password: "password123",
			},
			field:       "name",
			expectedMsg: "The name must have at least 3 characters",
		},
		{
			name: "Email field custom message",
			input: TestStruct{
				ID:       1,
				Name:     "John",
				Email:    "not-an-email", // Invalid email
				Password: "password123",
			},
			field:       "email",
			expectedMsg: "Please enter a valid email address",
		},
		{
			name: "Min value field custom message (Age specific)",
			input: TestStruct{
				ID:       1,
				Name:     "John",
				Email:    "john@example.com",
				Age:      16, // Too young
				Password: "password123",
			},
			field:       "age",
			expectedMsg: "You must be at least 18 years old",
		},
		{
			name: "Max value field custom message (Age specific)",
			input: TestStruct{
				ID:       1,
				Name:     "John",
				Email:    "john@example.com",
				Age:      130, // Too old
				Password: "password123",
			},
			field:       "age",
			expectedMsg: "You cannot be older than 120 years",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateStruct(context.Background(), tt.input)
			require.NoError(t, err)
			assert.False(t, result.Valid)

			// Check that the specific field has the expected error message
			reason, exists := result.Failed[tt.field]
			assert.True(t, exists, "Field %s should have an error", tt.field)
			assert.Equal(t, tt.expectedMsg, reason.Message,
				"Error message for field %s should match expected", tt.field)
		})
	}
}
