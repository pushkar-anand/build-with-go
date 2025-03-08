package validator

import (
	"fmt"
	"github.com/go-playground/validator/v10"
)

type (
	ValidationFunc func(fl validator.FieldLevel) bool
)

func (v *Validator) registerCustomTags(rules map[string]ValidationFunc) error {
	for tag, vFn := range rules {
		err := v.validator.RegisterValidation(tag, validator.Func(vFn))
		if err != nil {
			return fmt.Errorf("failed to register custom tag '%s': %w", tag, err)
		}
	}

	return nil
}
