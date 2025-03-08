package validator

import (
	"github.com/go-playground/validator/v10"
	"reflect"
	"strings"
	"sync"
)

type Validator struct {
	rules     map[string]ValidationFunc
	validator *validator.Validate
}

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
