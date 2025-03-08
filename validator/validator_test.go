package validator

import (
	"context"
	"sync"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structures for validation
type (
	TestStruct struct {
		ID       int     `json:"id" validate:"required"`
		Name     string  `json:"name" validate:"required,min=3"`
		Email    string  `json:"email" validate:"required,email"`
		Age      int     `json:"age" validate:"min=18,max=120"`
		Password string  `json:"password" validate:"required,min=8"`
		OptField *string `json:"opt_field"`
	}
)

// TestValidator_ValidateStruct_basic tests basic validation functionality
func TestValidator_ValidateStruct_basic(t *testing.T) {
	resetValidator()

	// Create a validator with a custom tag
	v, err := New()
	require.NoError(t, err)
	require.NotNil(t, v)

	// Table-driven tests for different validation scenarios
	tests := []struct {
		name          string
		input         TestStruct
		expectedValid bool
		expectedErrs  map[string]bool // Field -> should have error
	}{
		{
			name: "Valid struct",
			input: TestStruct{
				ID:       1,
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      30,
				Password: "securepassword",
			},
			expectedValid: true,
			expectedErrs:  nil,
		},
		{
			name: "Missing required fields",
			input: TestStruct{
				Email:    "john@example.com",
				Age:      30,
				Password: "securepassword",
			},
			expectedValid: false,
			expectedErrs: map[string]bool{
				"id":   true,
				"name": true,
			},
		},
		{
			name: "Invalid email",
			input: TestStruct{
				ID:       1,
				Name:     "John Doe",
				Email:    "not-an-email",
				Age:      30,
				Password: "securepassword",
			},
			expectedValid: false,
			expectedErrs: map[string]bool{
				"email": true,
			},
		},
		{
			name: "Name too short",
			input: TestStruct{
				ID:       1,
				Name:     "Jo", // Too short
				Email:    "john@example.com",
				Age:      30,
				Password: "securepassword",
			},
			expectedValid: false,
			expectedErrs: map[string]bool{
				"name": true,
			},
		},
		{
			name: "Age too young",
			input: TestStruct{
				ID:       1,
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      16, // Too young
				Password: "securepassword",
			},
			expectedValid: false,
			expectedErrs: map[string]bool{
				"age": true,
			},
		},
		{
			name: "Password too short",
			input: TestStruct{
				ID:       1,
				Name:     "John Doe",
				Email:    "john@example.com",
				Age:      30,
				Password: "short", // Too short
			},
			expectedValid: false,
			expectedErrs: map[string]bool{
				"password": true,
			},
		},
		{
			name: "Multiple validation errors",
			input: TestStruct{
				Email:    "not-an-email",
				Password: "pwd",
			},
			expectedValid: false,
			expectedErrs: map[string]bool{
				"id":       true,
				"name":     true,
				"email":    true,
				"age":      true,
				"password": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateStruct(context.Background(), tt.input)
			require.NoError(t, err, "Validation should not return an error")

			assert.Equal(t, tt.expectedValid, result.Valid, "Validation result mismatch")

			if !tt.expectedValid {
				assert.NotNil(t, result.Failed, "Failed map should not be nil")

				// Check each expected error field
				for field, shouldHaveError := range tt.expectedErrs {
					if shouldHaveError {
						assert.Contains(t, result.Failed, field, "Field %s should have an error", field)
					} else {
						assert.NotContains(t, result.Failed, field, "Field %s should not have an error", field)
					}
				}

				// Check no unexpected fields have errors
				for field := range result.Failed {
					_, expected := tt.expectedErrs[field]
					assert.True(t, expected, "Unexpected error for field %s", field)
				}
			} else {
				// If valid, no fields should have errors
				assert.Empty(t, result.Failed, "No fields should have errors for valid input")
			}
		})
	}
}

var customTags = map[string]ValidationFunc{
	"custom_alpha": func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		for _, r := range s {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
				return false
			}
		}
		return true
	},
	"custom_numeric": func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		for _, r := range s {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	},
	"custom_non_empty": func(fl validator.FieldLevel) bool {
		return fl.Field().String() != ""
	},
}

// TestValidator_ValidateStruct_customTags tests the custom validation tags functionality
func TestValidator_ValidateStruct_customTags(t *testing.T) {
	resetValidator()

	type CustomTagsStruct struct {
		AlphaField    string `json:"alpha_field" validate:"custom_alpha"`
		NumericField  string `json:"numeric_field" validate:"custom_numeric"`
		NonEmptyField string `json:"non_empty_field" validate:"custom_non_empty"`
	}

	v, err := New(WithCustomTags(customTags))
	require.NoError(t, err)
	require.NotNil(t, v)

	tests := []struct {
		name            string
		input           CustomTagsStruct
		expectedValid   bool
		expectedInvalid []string // Fields expected to be invalid
	}{
		{
			name: "All valid fields",
			input: CustomTagsStruct{
				AlphaField:    "onlyletters",
				NumericField:  "12345",
				NonEmptyField: "notempty",
			},
			expectedValid:   true,
			expectedInvalid: nil,
		},
		{
			name: "Mixed valid and invalid fields",
			input: CustomTagsStruct{
				AlphaField:    "contains123", // Invalid - contains numbers
				NumericField:  "12345",
				NonEmptyField: "", // Invalid - empty
			},
			expectedValid:   false,
			expectedInvalid: []string{"alpha_field", "non_empty_field"},
		},
		{
			name: "All invalid fields",
			input: CustomTagsStruct{
				AlphaField:    "contains123",
				NumericField:  "123abc",
				NonEmptyField: "",
			},
			expectedValid:   false,
			expectedInvalid: []string{"alpha_field", "numeric_field", "non_empty_field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run validation
			result, err := v.ValidateStruct(context.Background(), tt.input)
			require.NoError(t, err, "Validation should not return an error")
			assert.Equal(t, tt.expectedValid, result.Valid, "Validation result mismatch")

			if !tt.expectedValid {
				// Check that expected invalid fields have errors
				for _, field := range tt.expectedInvalid {
					assert.Contains(t, result.Failed, field, "Field %s should have validation error", field)
				}

				// Check that only expected fields have errors
				assert.Equal(t, len(tt.expectedInvalid), len(result.Failed),
					"Number of validation errors should match expected")
			}
		})
	}
}

// Reset the validator singleton for tests
func resetValidator() {
	vGlobal = nil
	vErr = nil
	once = sync.Once{}
}
