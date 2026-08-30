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
	defaultValidator *Validator
	defaultErr       error
	defaultOnce      sync.Once
)

// New returns a Validator configured by opts.
//
// Each call builds a separate Validator, so options always take effect and one
// caller's custom rules cannot leak into another's.
func New(opts ...Option) (*Validator, error) {
	v := &Validator{
		validator: buildValidator(),
		rules:     make(map[string]ValidationFunc),
		messages:  maps.Clone(DefaultMessageMap),
	}

	for _, opt := range opts {
		opt.apply(v)
	}

	// We have to register the custom tags before using them.
	err := v.registerCustomTags(v.rules)
	if err != nil {
		return nil, err
	}

	return v, nil
}

// Default returns a Validator with no custom rules or messages, built once and
// shared by every caller. Use New when the validator needs options.
func Default() (*Validator, error) {
	defaultOnce.Do(func() {
		defaultValidator, defaultErr = New()
	})

	return defaultValidator, defaultErr
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

// ValidateRequest reports why s is invalid, keyed by field name, or nil when s
// is valid.
//
// It adapts Validator to the interface http/request expects, which is what lets
// that package validate without depending on this one.
func (v *Validator) ValidateRequest(ctx context.Context, s any) (map[string]any, error) {
	result, err := v.ValidateStruct(ctx, s)
	if err != nil {
		return nil, err
	}

	if result.Valid {
		return nil, nil
	}

	problems := make(map[string]any, len(result.Failed))
	for field, reason := range result.Failed {
		problems[field] = reason
	}

	return problems, nil
}

func (v *Validator) parseError(err error) (*Result, error) {
	var (
		invalidErr     *validator.InvalidValidationError
		validationErrs validator.ValidationErrors
	)

	switch {
	case err == nil:
		return &Result{Valid: true}, nil
	case errors.As(err, &invalidErr):
		return nil, fmt.Errorf("validation failed: %w", invalidErr)
	case errors.As(err, &validationErrs):
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
