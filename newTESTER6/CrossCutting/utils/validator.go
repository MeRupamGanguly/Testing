// Package validation provides custom validation rules for Gin.
package utils

import (
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// RegisterCustomValidations adds custom validation rules and JSON tag name mapping
// to Gin's default validator. Call this once during application startup.
func RegisterCustomValidations() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// Use JSON field names in error messages instead of struct field names.
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})

		// Register e‑commerce specific validations.
		_ = v.RegisterValidation("sku", validateSKU)
		_ = v.RegisterValidation("price", validatePrice)
		_ = v.RegisterValidation("phone", validatePhone)
	}
}

// Validate is a standalone helper that uses the same custom rules.
// Useful for programmatic validation outside of Gin binding.
func Validate(data interface{}) map[string]string {
	v := validator.New()
	// Re‑register the same custom rules.
	_ = v.RegisterValidation("sku", validateSKU)
	_ = v.RegisterValidation("price", validatePrice)
	_ = v.RegisterValidation("phone", validatePhone)

	err := v.Struct(data)
	if err == nil {
		return nil
	}

	validationErrors := err.(validator.ValidationErrors)
	errors := make(map[string]string)
	for _, e := range validationErrors {
		errors[e.Field()] = formatError(e)
	}
	return errors
}

// Custom validation functions
func validateSKU(fl validator.FieldLevel) bool {
	sku := fl.Field().String()
	if len(sku) < 6 || len(sku) > 20 {
		return false
	}
	for _, r := range sku {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func validatePrice(fl validator.FieldLevel) bool {
	price := fl.Field().Float()
	return price > 0
}

func validatePhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	return strings.HasPrefix(phone, "+") && len(phone) >= 10 && len(phone) <= 15
}

func formatError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + e.Param() + " characters"
	case "max":
		return "must be at most " + e.Param() + " characters"
	case "sku":
		return "must be a valid SKU (6-20 alphanumeric)"
	case "price":
		return "must be a valid positive price"
	case "phone":
		return "must be a valid phone number starting with +"
	default:
		return "failed validation for " + e.Tag()
	}
}
