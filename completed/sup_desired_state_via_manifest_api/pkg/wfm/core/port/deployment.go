package port

import (
	"context"
	"skeleton/pkg/wfm/core/domain"
)

type DeploymentRepository interface {
	UpsertDeployments(ctx context.Context, deviceId string, updateFn func(manifest *domain.ApplicationDeploymentManifest) error) error
	GetDeploymentManifest(ctx context.Context, deviceId string) (*domain.ApplicationDeploymentManifest, error)
	GetDeployment(ctx context.Context, deviceId, deploymentId, digest string) (*domain.ApplicationDeployment, error)
	GetBundle(ctx context.Context, deviceId, digest string) ([]byte, error)
}

type DeploymentService interface {
	CreateDeployment(ctx context.Context, deviceId string, descriptor []byte) (*domain.ApplicationDeployment, error)
	UpdateDeployment(ctx context.Context, deviceId, deploymentId string, descriptor []byte) (*domain.ApplicationDeployment, error)
	DeleteDeployment(ctx context.Context, deviceId, deploymentId string) error
	GetDeploymentManifest(ctx context.Context, deviceId string) (*domain.ApplicationDeploymentManifest, error)
	GetDeployment(ctx context.Context, deviceId, deploymentId, digest string) (*domain.ApplicationDeployment, error)
	GetBundle(ctx context.Context, deviceId, digest string) ([]byte, error)
}
