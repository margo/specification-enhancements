package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// Identities is an identity.Repository backed by the unified store's
// *sql.DB. Obtain via (*Store).Identities().
type Identities struct {
	db *sql.DB
}

// Identities returns the identity.Repository adapter.
func (s *Store) Identities() identity.Repository { return &Identities{db: s.db} }

// LookupByESI implements identity.Repository.
func (i *Identities) LookupByESI(ctx context.Context, esi []byte) (identity.ESIBinding, identity.LDI, error) {
	const q = `
		SELECT b.esi, b.ldi_uuid, b.method, b.status, b.bound_at, COALESCE(b.retired_at, ''),
		       l.uuid, l.spiffe_id, l.status, l.created_at, l.updated_at
		  FROM esi_bindings b
		  JOIN ldis l ON l.uuid = b.ldi_uuid
		 WHERE b.esi = ?
	`
	row := i.db.QueryRowContext(ctx, q, esi)
	var b identity.ESIBinding
	var l identity.LDI
	var boundAt, retiredAt, createdAt, updatedAt string
	err := row.Scan(&b.ESI, &b.LDIUUID, &b.Method, &b.Status, &boundAt, &retiredAt,
		&l.UUID, &l.SPIFFEID, &l.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.ESIBinding{}, identity.LDI{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.ESIBinding{}, identity.LDI{}, err
	}
	b.BoundAt = mustParseTime(boundAt)
	if retiredAt != "" {
		b.RetiredAt = mustParseTime(retiredAt)
	}
	l.CreatedAt = mustParseTime(createdAt)
	l.UpdatedAt = mustParseTime(updatedAt)
	return b, l, nil
}

// CreateLDIAndBind implements identity.Repository.
func (i *Identities) CreateLDIAndBind(ctx context.Context, esi []byte, method string, trustDomain string) (identity.LDI, error) {
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return identity.LDI{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().UTC()
	id := uuid.New().String()
	spiffeID := fmt.Sprintf("spiffe://%s/margo/device/%s", trustDomain, id)

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO ldis (uuid, spiffe_id, status, created_at, updated_at) VALUES (?, ?, 'active', ?, ?)`,
		id, spiffeID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return identity.LDI{}, fmt.Errorf("insert ldi: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO esi_bindings (esi, ldi_uuid, method, status, bound_at) VALUES (?, ?, ?, 'active', ?)`,
		esi, id, method, now.Format(time.RFC3339Nano)); err != nil {
		return identity.LDI{}, fmt.Errorf("insert binding: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return identity.LDI{}, err
	}
	return identity.LDI{
		UUID:      id,
		SPIFFEID:  spiffeID,
		Status:    identity.LDIStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Rebind implements identity.Repository.
func (i *Identities) Rebind(ctx context.Context, ldiUUID string, esi []byte, method string) (identity.ESIBinding, error) {
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return identity.ESIBinding{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	// Confirm the LDI exists.
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ldis WHERE uuid = ?`, ldiUUID).Scan(&count); err != nil {
		return identity.ESIBinding{}, err
	}
	if count == 0 {
		return identity.ESIBinding{}, identity.ErrNotFound
	}

	now := time.Now().UTC()

	// Retire any currently active binding for this LDI.
	if _, err := tx.ExecContext(ctx,
		`UPDATE esi_bindings SET status = 'retired', retired_at = ? WHERE ldi_uuid = ? AND status = 'active'`,
		now.Format(time.RFC3339Nano), ldiUUID); err != nil {
		return identity.ESIBinding{}, fmt.Errorf("retire previous: %w", err)
	}

	// Insert the new active binding.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO esi_bindings (esi, ldi_uuid, method, status, bound_at) VALUES (?, ?, ?, 'active', ?)`,
		esi, ldiUUID, method, now.Format(time.RFC3339Nano)); err != nil {
		return identity.ESIBinding{}, fmt.Errorf("insert new binding: %w", err)
	}

	// Touch the LDI's updated_at.
	if _, err := tx.ExecContext(ctx,
		`UPDATE ldis SET updated_at = ? WHERE uuid = ?`, now.Format(time.RFC3339Nano), ldiUUID); err != nil {
		return identity.ESIBinding{}, err
	}

	if err := tx.Commit(); err != nil {
		return identity.ESIBinding{}, err
	}
	return identity.ESIBinding{
		ESI:     esi,
		LDIUUID: ldiUUID,
		Method:  method,
		Status:  identity.ESIBindingActive,
		BoundAt: now,
	}, nil
}

// GetLDI implements identity.Repository.
func (i *Identities) GetLDI(ctx context.Context, id string) (identity.LDI, error) {
	const q = `SELECT uuid, spiffe_id, status, created_at, updated_at FROM ldis WHERE uuid = ?`
	var l identity.LDI
	var createdAt, updatedAt string
	err := i.db.QueryRowContext(ctx, q, id).Scan(&l.UUID, &l.SPIFFEID, &l.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.LDI{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.LDI{}, err
	}
	l.CreatedAt = mustParseTime(createdAt)
	l.UpdatedAt = mustParseTime(updatedAt)
	return l, nil
}

// GetLDIBySPIFFEID implements identity.Repository.
func (i *Identities) GetLDIBySPIFFEID(ctx context.Context, spiffeID string) (identity.LDI, error) {
	const q = `SELECT uuid, spiffe_id, status, created_at, updated_at FROM ldis WHERE spiffe_id = ?`
	var l identity.LDI
	var createdAt, updatedAt string
	err := i.db.QueryRowContext(ctx, q, spiffeID).Scan(&l.UUID, &l.SPIFFEID, &l.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.LDI{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.LDI{}, err
	}
	l.CreatedAt = mustParseTime(createdAt)
	l.UpdatedAt = mustParseTime(updatedAt)
	return l, nil
}

// ListBindings implements identity.Repository.
func (i *Identities) ListBindings(ctx context.Context, ldiUUID string) ([]identity.ESIBinding, error) {
	rows, err := i.db.QueryContext(ctx,
		`SELECT esi, ldi_uuid, method, status, bound_at, COALESCE(retired_at, '') FROM esi_bindings WHERE ldi_uuid = ? ORDER BY bound_at ASC`,
		ldiUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []identity.ESIBinding
	for rows.Next() {
		var b identity.ESIBinding
		var boundAt, retiredAt string
		if err := rows.Scan(&b.ESI, &b.LDIUUID, &b.Method, &b.Status, &boundAt, &retiredAt); err != nil {
			return nil, err
		}
		b.BoundAt = mustParseTime(boundAt)
		if retiredAt != "" {
			b.RetiredAt = mustParseTime(retiredAt)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// SetLDIStatus implements identity.Repository.
func (i *Identities) SetLDIStatus(ctx context.Context, id string, status identity.LDIStatus) error {
	res, err := i.db.ExecContext(ctx,
		`UPDATE ldis SET status = ?, updated_at = ? WHERE uuid = ?`,
		string(status), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func mustParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}
