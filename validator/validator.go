package validator

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"maps"
	"reflect"
	"strings"
	"sync"
)

type (
	Validator struct {
		rules     map[string]ValidationFunc
		messages  map[string]MessageFunc
		validator *validator.Validate
	}

	Reason struct {
		Value   any    `json:"value"`
		Rule    string `json:"rule"`
		Message string `json:"message"`
	}

	Result struct {
		Valid  bool
		Failed map[string]Reason
	}
)

var (
	vGlobal *Validator
	vErr    error
	once    sync.Once
)

// New returns a new Validator.
//
// It creates or reuses an existing validator if already created.
func New(opts ...Option) (*Validator, error) {
	once.Do(func() {
		v := buildValidator()

		vl := &Validator{
			validator: v,
			rules:     make(map[string]ValidationFunc),
			messages:  maps.Clone(DefaultMessageMap),
		}

		for _, opt := range opts {
			opt.apply(vl)
		}

		// We have to register the custom tags before using them.
		err := vl.registerCustomTags(vl.rules)
		if err == nil {
			vGlobal = vl
		}

		vErr = err
	})

	return vGlobal, vErr
}

// buildValidator builds the validator.Validate.
// It also adds the function to read JSON tags
// from the struct to use it for reposting errors.
// Reference: https://github.com/go-playground/validator/blob/58d5778b183e89cc374ca4ebbf06da1eed088a63/_examples/struct-level/main.go#L37
func buildValidator() *validator.Validate {
	v := validator.New(
		validator.WithRequiredStructEnabled(),
	)

	// register function to get tag name from JSON tags.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "" {
			name = strings.SplitN(fld.Tag.Get("schema"), ",", 2)[0]
		}

		if name == "-" {
			return ""
		}

		return name
	})

	return v
}

func (v *Validator) ValidateStruct(ctx context.Context, s any) (*Result, error) {
	err := v.validator.StructCtx(ctx, s)
	if err != nil {
		return v.parseError(err)
	}

	return &Result{Valid: true}, nil
}

func (v *Validator) parseError(err error) (*Result, error) {
	var (
		invalidErr     *validator.InvalidValidationError
		validationErrs validator.ValidationErrors
	)

	switch {
	case err == nil:
		return &Result{Valid: true}, nil
	case errors.As(err, &invalidErr), errors.Is(err, invalidErr):
		return nil, fmt.Errorf("validation failed: %w", invalidErr)
	case errors.Is(err, &validationErrs):
		failures := make(map[string]Reason)

		for _, validationErr := range validationErrs {
			field := validationErr.Field()
			tag := validationErr.ActualTag()

			failures[field] = Reason{
				Value:   validationErr.Value(),
				Rule:    tag,
				Message: v.createUserFriendlyMessage(field, tag, validationErr),
			}
		}

		return &Result{Valid: false, Failed: failures}, nil
	default:
		return nil, fmt.Errorf("validation failed with unexpected error: %w", err)
	}
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
