package bootstrap_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
)

type fakeValidator struct {
	esi []byte
	err error
}

func (f *fakeValidator) Validate(_ context.Context, _ *http.Request, cred common.BootstrapCredentialDTO, _, _ string) (bootstrap.ValidationOutcome, error) {
	if f.err != nil {
		return bootstrap.ValidationOutcome{}, f.err
	}
	return bootstrap.ValidationOutcome{
		Pending: &bootstrap.PendingEnrollment{
			VB: bootstrap.VerifiedBootstrap{
				Method: cred.Method,
				ESI:    f.esi,
			},
		},
	}, nil
}

func TestRegistry_LookupKnown(t *testing.T) {
	reg := bootstrap.NewRegistry()
	reg.Register(common.BootstrapMethodFactoryCertMTLS, &fakeValidator{esi: []byte{1, 2, 3}})

	v, err := reg.Lookup(common.BootstrapMethodFactoryCertMTLS)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if v == nil {
		t.Fatal("returned validator is nil")
	}
}

func TestRegistry_LookupUnknown(t *testing.T) {
	reg := bootstrap.NewRegistry()
	_, err := reg.Lookup("urn:margo:bootstrap:nope:v9")
	if !errors.Is(err, bootstrap.ErrUnsupportedMethod) {
		t.Errorf("err = %v, want ErrUnsupportedMethod", err)
	}
}

func TestRegistry_Methods_IsSorted(t *testing.T) {
	reg := bootstrap.NewRegistry()
	reg.Register(common.BootstrapMethodFactoryCertJWT, &fakeValidator{})
	reg.Register(common.BootstrapMethodFactoryCertMTLS, &fakeValidator{})
	got := reg.Methods()
	if len(got) != 2 {
		t.Fatalf("methods len = %d, want 2", len(got))
	}
	if got[0] >= got[1] {
		t.Errorf("methods must be sorted: %v", got)
	}
}
