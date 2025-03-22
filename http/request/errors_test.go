package request

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net/http"
	"strings"
	"testing"
)

func Test_parseReadError_JSONErrors(t *testing.T) {
	type Dummy struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name         string
		jsonInput    string
		expectedCode int
	}{
		{
			name:         "Unterminated JSON object",
			jsonInput:    `{"name": "John", "age": 30`,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Invalid type - string as int",
			jsonInput:    `{"name": "John", "age": "thirty"}`,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Unknown field with strict mode",
			jsonInput:    `{"name": "John", "age": 30, "unknown_field": true}`,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Complex nested invalid JSON",
			jsonInput:    `{"name": "John", "age": 30, "address": {"street": 123, "city": true}}`,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Empty string",
			jsonInput:    ``,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decoder := json.NewDecoder(strings.NewReader(tc.jsonInput))
			decoder.DisallowUnknownFields() // To trigger unknown field errors

			var p Dummy
			err := decoder.Decode(&p)

			readErr := parseReadError(err)
			assert.NotNil(t, readErr)
			assert.Equal(t, tc.expectedCode, readErr.HTTPStatusCode)
			assert.ErrorIs(t, err, readErr.UnderlyingErr)
		})
	}
}
