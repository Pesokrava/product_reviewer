package validator

import (
	"github.com/go-playground/validator/v10"
)

// Shared validator instance to avoid creating multiple instances
var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Get returns the shared validator instance
func Get() *validator.Validate {
	return validate
}
