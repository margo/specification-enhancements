package jwtexchange_test

import (
	"errors"
	"testing"

	"github.com/margo/miaf-poc/pkg/mis/jwtexchange"
)

func TestValidateAudience_Empty(t *testing.T) {
	if err := jwtexchange.ValidateAudience(nil); !errors.Is(err, jwtexchange.ErrInvalidAudience) {
		t.Errorf("nil aud: err = %v, want ErrInvalidAudience", err)
	}
	if err := jwtexchange.ValidateAudience([]string{}); !errors.Is(err, jwtexchange.ErrInvalidAudience) {
		t.Errorf("empty aud: err = %v, want ErrInvalidAudience", err)
	}
}

func TestValidateAudience_NonURL(t *testing.T) {
	if err := jwtexchange.ValidateAudience([]string{""}); !errors.Is(err, jwtexchange.ErrInvalidAudience) {
		t.Error("empty string element should fail")
	}
	if err := jwtexchange.ValidateAudience([]string{"::not a url"}); !errors.Is(err, jwtexchange.ErrInvalidAudience) {
		t.Error("garbage URL should fail")
	}
}

func TestValidateAudience_Valid(t *testing.T) {
	if err := jwtexchange.ValidateAudience([]string{"https://dfm/", "urn:margo:thing"}); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
