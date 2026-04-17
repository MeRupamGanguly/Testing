package validation

import (
	"fmt"
	"strings"
)

// ErrorList collects multiple validation errors.
type ErrorList []error

func (e ErrorList) Error() string {
	if len(e) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("validation failed: ")
	for i, err := range e {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// ValidatePositive returns an error if val <= 0.
func ValidatePositive(val int64, fieldName string) error {
	if val <= 0 {
		return fmt.Errorf("%s must be positive", fieldName)
	}
	return nil
}

// ValidateNotNil returns an error if val is nil.
func ValidateNotNil(val interface{}, fieldName string) error {
	if val == nil {
		return fmt.Errorf("%s cannot be nil", fieldName)
	}
	return nil
}

// ValidateNonZeroString returns error if string is empty.
func ValidateNonZeroString(s, fieldName string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	return nil
}

// ValidateMinLength checks string length.
func ValidateMinLength(s, fieldName string, min int) error {
	if len(strings.TrimSpace(s)) < min {
		return fmt.Errorf("%s must be at least %d characters", fieldName, min)
	}
	return nil
}

// ValidateCurrency checks if a currency code is valid (using the money package).
// We avoid direct import here to keep decoupled; the user can pass a validation func.
type CurrencyValidator func(string) bool

// ValidateCurrencyCode uses the provided validator.
func ValidateCurrencyCode(code string, validator CurrencyValidator, fieldName string) error {
	if !validator(code) {
		return fmt.Errorf("%s '%s' is not a supported currency", fieldName, code)
	}
	return nil
}

// ValidateStruct runs a set of validators on a struct (simple example).
func ValidateStruct(obj interface{}, validators map[string]func() error) ErrorList {
	var errs ErrorList
	for field, validator := range validators {
		if err := validator(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", field, err))
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}
