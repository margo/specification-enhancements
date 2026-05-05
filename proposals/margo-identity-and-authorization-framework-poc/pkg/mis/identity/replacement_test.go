package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

func TestNoReplacements_AlwaysRejects(t *testing.T) {
	_, err := identity.NoReplacements{}.Validate(context.Background(), "anything", time.Now())
	if !errors.Is(err, identity.ErrReplacementNotAuthorized) {
		t.Errorf("NoReplacements.Validate = %v, want ErrReplacementNotAuthorized", err)
	}
}
