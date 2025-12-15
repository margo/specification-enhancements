package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"skeleton/pkg/wfm/adapter/persistence/sqlitedb"
	"skeleton/pkg/wfm/adapter/persistence/sqlitedb/db"
	"skeleton/pkg/wfm/core/domain"
)

type DeploymentRepository struct {
	ds *sqlitedb.DataStore
}

func NewDeploymentRepository(ds *sqlitedb.DataStore) *DeploymentRepository {
	return &DeploymentRepository{
		ds: ds,
	}
}

func (dr *DeploymentRepository) UpsertDeployments(ctx context.Context, deviceId string, updateFn func(manifest *domain.ApplicationDeploymentManifest) error) (err error) {
	tx, err := dr.ds.BeginTransaction(ctx)
	if err != nil {
		return errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to start transaction: %w", err))
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	qtx := dr.ds.Queries.WithTx(tx)

	if err = ensureDeviceExists(ctx, qtx, deviceId); err != nil {
		return err
	}

	manifest, err := dr.loadManifestWithDeployments(ctx, deviceId, qtx)
	if err != nil {
		if errors.Is(err, domain.ErrManifestNotFound) {
			manifest = &domain.ApplicationDeploymentManifest{Version: 1}
		} else {
			return err
		}
	}
	originalDeploymentIDs := make(map[string]struct{}, len(manifest.Deployments))
	for _, deployment := range manifest.Deployments {
		originalDeploymentIDs[deployment.Id] = struct{}{}
	}

	// Let the caller apply mutations
	if err = updateFn(manifest); err != nil {
		return errors.Join(domain.ErrInternal, fmt.Errorf("db: update callback failed: %w", err))
	}
	currentDeploymentIDs := make(map[string]struct{}, len(manifest.Deployments))
	for _, deployment := range manifest.Deployments {
		currentDeploymentIDs[deployment.Id] = struct{}{}
	}
	for id := range originalDeploymentIDs {
		if _, ok := currentDeploymentIDs[id]; !ok {
			if err = qtx.DeleteDeployment(ctx, db.DeleteDeploymentParams{
				DeviceID: deviceId,
				ID:       id,
			}); err != nil {
				return errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to delete deployment: %w", err))
			}
		}
	}

	if len(manifest.BundleArchive) > 0 && manifest.BundleDigest != "" {
		if err = qtx.InsertBundleBlob(ctx, db.InsertBundleBlobParams{
			Digest:  manifest.BundleDigest,
			Archive: manifest.BundleArchive,
		}); err != nil {
			return errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to persist bundle blob: %w", err))
		}
	}
	var bundleDigest sql.NullString
	if manifest.BundleDigest != "" {
		bundleDigest = sql.NullString{String: manifest.BundleDigest, Valid: true}
	}
	if err = qtx.UpsertManifest(ctx, db.UpsertManifestParams{
		Version:      int64(manifest.Version),
		BundleDigest: bundleDigest,
		DeviceID:     deviceId,
	}); err != nil {
		return errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to upsert manifest: %w", err))
	}
	for _, deployment := range manifest.Deployments {
		if err = qtx.InsertDeploymentBlob(ctx, db.InsertDeploymentBlobParams{
			Digest:     deployment.DescriptorDigest,
			Descriptor: deployment.Descriptor,
		}); err != nil {
			return errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to persist deployment blob: %w", err))
		}
		if err = qtx.UpsertDeployment(ctx, db.UpsertDeploymentParams{
			ID:               deployment.Id,
			DescriptorDigest: deployment.DescriptorDigest,
			DeviceID:         deviceId,
		}); err != nil {
			return errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to upsert deployment: %w", err))
		}
	}

	if err = tx.Commit(); err != nil {
		return errors.Join(domain.ErrInternal, fmt.Errorf("db: commit failed: %w", err))
	}
	return nil
}

func (dr *DeploymentRepository) GetDeploymentManifest(ctx context.Context, deviceId string) (_ *domain.ApplicationDeploymentManifest, err error) {
	tx, err := dr.ds.BeginTransaction(ctx)
	if err != nil {
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to start transaction: %w", err))
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	qtx := dr.ds.Queries.WithTx(tx)

	if err = ensureDeviceExists(ctx, qtx, deviceId); err != nil {
		return nil, err
	}

	manifest, err := dr.loadManifestWithDeployments(ctx, deviceId, qtx)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: commit failed: %w", err))
	}
	return manifest, nil
}

func (dr *DeploymentRepository) GetDeployment(ctx context.Context, deviceId, deploymentId, digest string) (_ *domain.ApplicationDeployment, err error) {
	tx, err := dr.ds.BeginTransaction(ctx)
	if err != nil {
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to start transaction: %w", err))
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	qtx := dr.ds.Queries.WithTx(tx)

	if err = ensureDeviceExists(ctx, qtx, deviceId); err != nil {
		return nil, err
	}

	// note that there is a trust gap: any device can request any deployment blob. Production code would need to
	// maintain the device<>blob relationship even for historical deployments.
	blob, err := qtx.GetDeploymentBlobByDigest(ctx, digest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrDeploymentNotFound
		}
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to retrieve deployment blob: %w", err))
	}

	deployment := &domain.ApplicationDeployment{
		Id:               deploymentId,
		Descriptor:       blob.Descriptor,
		DescriptorDigest: blob.Digest,
	}

	if err = tx.Commit(); err != nil {
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: commit failed: %w", err))
	}
	return deployment, nil
}

func (dr *DeploymentRepository) GetBundle(ctx context.Context, deviceId, digest string) (_ []byte, err error) {
	tx, err := dr.ds.BeginTransaction(ctx)
	if err != nil {
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to start transaction: %w", err))
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	qtx := dr.ds.Queries.WithTx(tx)

	if err = ensureDeviceExists(ctx, qtx, deviceId); err != nil {
		return nil, err
	}

	// note that there is a trust gap: any device can request any bundle blob. Production code would need to
	// maintain the device<>blob relationship even for historical deployments.
	row, err := qtx.GetBundleBlobByDigest(ctx, digest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrBundleNotFound
		}
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to retrieve bundle blob: %w", err))
	}

	if err = tx.Commit(); err != nil {
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: commit failed: %w", err))
	}
	return row.Archive, nil
}

func (dr *DeploymentRepository) loadManifestWithDeployments(ctx context.Context, deviceId string, qtx *db.Queries) (*domain.ApplicationDeploymentManifest, error) {
	dbManifest, err := qtx.GetManifestByDeviceId(ctx, deviceId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrManifestNotFound
		}
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to retrieve manifest: %w", err))
	}
	dbDeployments, err := qtx.GetDeploymentsByDeviceId(ctx, deviceId)
	if err != nil {
		return nil, errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to retrieve deployments: %w", err))
	}
	deployments := make([]domain.ApplicationDeployment, len(dbDeployments))
	for i, dbDeployment := range dbDeployments {
		deployments[i] = domain.ApplicationDeployment{
			Id:               dbDeployment.ID,
			Descriptor:       dbDeployment.Descriptor,
			DescriptorDigest: dbDeployment.DescriptorDigest,
		}
	}
	bundleDigest := ""
	if dbManifest.BundleDigest.Valid {
		bundleDigest = dbManifest.BundleDigest.String
	}
	manifest := &domain.ApplicationDeploymentManifest{
		Version:      uint64(dbManifest.Version),
		BundleDigest: bundleDigest,
		Deployments:  deployments,
	}
	return manifest, nil
}

func ensureDeviceExists(ctx context.Context, qtx *db.Queries, deviceId string) error {
	if _, err := qtx.GetDeviceId(ctx, deviceId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrDeviceNotFound
		}
		return errors.Join(domain.ErrInternal, fmt.Errorf("db: failed to lookup device: %w", err))
	}
	return nil
}
