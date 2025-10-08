package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"skeleton/pkg/common"
	"skeleton/pkg/wfm/core/domain"
	"skeleton/pkg/wfm/core/port"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type DeploymentService struct {
	deploymentRepo port.DeploymentRepository
	validate       *validator.Validate
}

func NewDeploymentService(deploymentRepo port.DeploymentRepository) *DeploymentService {
	return &DeploymentService{
		deploymentRepo: deploymentRepo,
		validate:       validator.New(),
	}
}

func (ds *DeploymentService) CreateDeployment(ctx context.Context, deviceId string, serializedDescriptor []byte) (*domain.ApplicationDeployment, error) {
	var descriptor common.ApplicationDeploymentDescriptor
	if err := yaml.Unmarshal(serializedDescriptor, &descriptor); err != nil {
		return nil, errors.Join(domain.ErrInvalidDeploymentDescriptor, fmt.Errorf("svc: failed to unmarshal ApplicationDeployment YAML: %w", err))
	}
	if err := ds.validate.Struct(descriptor); err != nil {
		return nil, errors.Join(domain.ErrInvalidDeploymentDescriptor, fmt.Errorf("svc: failed to validate ApplicationDeployment YAML: %w", err))
	}

	// The deployment ID is embedded in the descriptor. Hence, we need to patch it in the
	// descriptor and serialize the descriptor with the changed deployment ID.
	descriptor.Metadata.Annotations.Id = uuid.New().String()
	rendered, err := yaml.Marshal(descriptor)
	if err != nil {
		return nil, errors.Join(domain.ErrInvalidDeploymentDescriptor, fmt.Errorf("svc: failed to marshal ApplicationDeployment YAML: %w", err))
	}

	// Persist the deployment and update the bundle
	applicationDeployment := domain.ApplicationDeployment{
		Id:               descriptor.Metadata.Annotations.Id,
		Descriptor:       rendered,
		DescriptorDigest: common.CalculateDigest(rendered),
	}
	err = ds.deploymentRepo.UpsertDeployments(ctx, deviceId, func(manifest *domain.ApplicationDeploymentManifest) error {
		// Add the deployment to the device's manifest
		manifest.Deployments = append(manifest.Deployments, applicationDeployment)

		return rebuildManifestBundle(manifest)
	})
	if err != nil {
		return nil, err
	}

	return &applicationDeployment, nil
}

func (ds *DeploymentService) UpdateDeployment(ctx context.Context, deviceId, deploymentId string, serializedDescriptor []byte) (*domain.ApplicationDeployment, error) {
	var descriptor common.ApplicationDeploymentDescriptor
	if err := yaml.Unmarshal(serializedDescriptor, &descriptor); err != nil {
		return nil, errors.Join(domain.ErrInvalidDeploymentDescriptor, fmt.Errorf("svc: failed to unmarshal ApplicationDeployment YAML: %w", err))
	}
	if err := ds.validate.Struct(descriptor); err != nil {
		return nil, errors.Join(domain.ErrInvalidDeploymentDescriptor, fmt.Errorf("svc: failed to validate ApplicationDeployment YAML: %w", err))
	}
	if descriptor.Metadata.Annotations.Id != "" && descriptor.Metadata.Annotations.Id != deploymentId {
		return nil, errors.Join(domain.ErrInvalidDeploymentDescriptor, fmt.Errorf("svc: descriptor deployment ID %q does not match path deployment ID %q", descriptor.Metadata.Annotations.Id, deploymentId))
	}
	descriptor.Metadata.Annotations.Id = deploymentId
	rendered, err := yaml.Marshal(descriptor)
	if err != nil {
		return nil, errors.Join(domain.ErrInvalidDeploymentDescriptor, fmt.Errorf("svc: failed to marshal ApplicationDeployment YAML: %w", err))
	}
	updatedDeployment := domain.ApplicationDeployment{
		Id:               deploymentId,
		Descriptor:       rendered,
		DescriptorDigest: common.CalculateDigest(rendered),
	}
	err = ds.deploymentRepo.UpsertDeployments(ctx, deviceId, func(manifest *domain.ApplicationDeploymentManifest) error {
		for i := range manifest.Deployments {
			if manifest.Deployments[i].Id == deploymentId {
				manifest.Deployments[i] = updatedDeployment
				return rebuildManifestBundle(manifest)
			}
		}
		return domain.ErrDeploymentNotFound
	})
	if err != nil {
		return nil, err
	}

	return &updatedDeployment, nil
}

func (ds *DeploymentService) DeleteDeployment(ctx context.Context, deviceId, deploymentId string) error {
	return ds.deploymentRepo.UpsertDeployments(ctx, deviceId, func(manifest *domain.ApplicationDeploymentManifest) error {
		idx := -1
		for i := range manifest.Deployments {
			if manifest.Deployments[i].Id == deploymentId {
				idx = i
				break
			}
		}
		if idx == -1 {
			return domain.ErrDeploymentNotFound
		}
		manifest.Deployments = append(manifest.Deployments[:idx], manifest.Deployments[idx+1:]...)
		return rebuildManifestBundle(manifest)
	})
}

func (ds *DeploymentService) GetDeploymentManifest(ctx context.Context, deviceId string) (*domain.ApplicationDeploymentManifest, error) {
	return ds.deploymentRepo.GetDeploymentManifest(ctx, deviceId)
}

func (ds *DeploymentService) GetDeployment(ctx context.Context, deviceId, deploymentId, digest string) (*domain.ApplicationDeployment, error) {
	return ds.deploymentRepo.GetDeployment(ctx, deviceId, deploymentId, digest)
}

func (ds *DeploymentService) GetBundle(ctx context.Context, deviceId, expectedDigest string) ([]byte, error) {
	archive, err := ds.deploymentRepo.GetBundle(ctx, deviceId, expectedDigest)
	if err != nil {
		return nil, err
	}
	if len(archive) == 0 {
		return nil, domain.ErrBundleNotFound
	}
	return archive, nil
}

type file struct {
	Name    string
	Content []byte
}

func createBundleArchive(files []file) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(file.Content); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func rebuildManifestBundle(manifest *domain.ApplicationDeploymentManifest) error {
	files := make([]file, 0, len(manifest.Deployments))
	for _, deployment := range manifest.Deployments {
		files = append(files, file{
			Name:    fmt.Sprintf("%s.yaml", deployment.Id),
			Content: deployment.Descriptor,
		})
	}

	previousDigest := manifest.BundleDigest
	if len(files) == 0 {
		manifest.BundleArchive = nil
		manifest.BundleDigest = ""
	} else {
		archive, err := createBundleArchive(files)
		if err != nil {
			return errors.Join(domain.ErrInternal, fmt.Errorf("svc: failed to create tar.gz bundle: %w", err))
		}
		manifest.BundleArchive = archive
		manifest.BundleDigest = common.CalculateDigest(archive)
	}

	if manifest.BundleDigest == previousDigest {
		logrus.WithField("deployments", len(manifest.Deployments)).Info("svc: manifest bundle unchanged; skipping version increment")
		return nil
	}

	manifest.Version = manifest.Version + 1
	return nil
}
