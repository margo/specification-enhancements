package identity

import (
	"context"
	"errors"
)

// ErrNotFound signals that the requested ESI or LDI is not present.
var ErrNotFound = errors.New("identity: not found")

// Repository persists LDI and ESI binding state. Implementations must be
// safe for concurrent use. All write operations touching both tables MUST be
// transactional.
type Repository interface {
	// LookupByESI returns the (active or retired) binding for the given ESI
	// together with the LDI it is/was bound to, or ErrNotFound if the ESI has
	// never been bound. Callers are expected to distinguish active vs retired
	// by inspecting ESIBinding.Status: the ESI-immutability invariant (SUP §4)
	// guarantees at most one binding row per ESI across all statuses.
	LookupByESI(ctx context.Context, esi []byte) (ESIBinding, LDI, error)

	// CreateLDIAndBind atomically creates a fresh Active LDI and binds the
	// given ESI (Active) to it. Returns the new LDI.
	CreateLDIAndBind(ctx context.Context, esi []byte, method string, trustDomain string) (LDI, error)

	// Rebind atomically binds a new ESI (Active) to an existing LDI, retiring
	// the previously active ESI on that LDI. The LDI is looked up by UUID.
	Rebind(ctx context.Context, ldiUUID string, esi []byte, method string) (ESIBinding, error)

	// GetLDI returns the LDI with the given UUID or ErrNotFound.
	GetLDI(ctx context.Context, uuid string) (LDI, error)

	// GetLDIBySPIFFEID returns the LDI whose SPIFFE ID matches, or
	// ErrNotFound. SPIFFE IDs are unique in the ldis table (UNIQUE
	// constraint), so at most one row is returned.
	GetLDIBySPIFFEID(ctx context.Context, spiffeID string) (LDI, error)

	// ListBindings returns all bindings (active + retired) for the LDI,
	// ordered by BoundAt ascending. Used for audit.
	ListBindings(ctx context.Context, ldiUUID string) ([]ESIBinding, error)

	// SetLDIStatus transitions the LDI's lifecycle status. The store
	// enforces the CHECK constraint on the column; callers pass one of
	// LDIStatusActive, LDIStatusRevoked, or LDIStatusRetired. Returns
	// ErrNotFound when no row with the given UUID exists.
	SetLDIStatus(ctx context.Context, uuid string, status LDIStatus) error
}
