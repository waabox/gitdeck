// internal/domain/errors_test.go
package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/waabox/gitdeck/internal/domain"
)

func TestErrUnauthorized_CanBeDetectedWithErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("gitlab API error: %w", domain.ErrUnauthorized)
	if !errors.Is(wrapped, domain.ErrUnauthorized) {
		t.Error("expected errors.Is to detect ErrUnauthorized in wrapped error")
	}
}
