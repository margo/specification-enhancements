package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"skeleton/pkg/common"
	"skeleton/pkg/wfm/core/domain"
	"skeleton/pkg/wfm/core/port"
	"strings"

	"github.com/sirupsen/logrus"
)

type DeploymentHandler struct {
	svc port.DeploymentService
}

func NewDeploymentHandler(svc port.DeploymentService) *DeploymentHandler {
	return &DeploymentHandler{
		svc,
	}
}

func (s *DeploymentHandler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	deviceId := r.PathValue("deviceId")

	descriptor, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"error":    err,
		}).Error("Failed to read HTTP body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	created, err := s.svc.CreateDeployment(r.Context(), deviceId, descriptor)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDeviceNotFound):
			logrus.WithFields(logrus.Fields{
				"deviceId": deviceId,
				"error":    err,
			}).Warn("Device not found")
			http.Error(w, "Device not found", http.StatusNotFound)
			return
		case errors.Is(err, domain.ErrInvalidDeploymentDescriptor):
			logrus.WithFields(logrus.Fields{
				"deviceId": deviceId,
				"error":    err,
			}).Warn("Invalid deployment descriptor")
			http.Error(w, "Invalid deployment descriptor", http.StatusBadRequest)
			return
		default:
			logrus.WithFields(logrus.Fields{
				"deviceId": deviceId,
				"error":    err,
			}).Error("Failed to create deployment")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", created.DescriptorDigest))
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Location", fmt.Sprintf("/api/v1/devices/%s/deployments/%s/%s", deviceId, created.Id, created.DescriptorDigest))
	w.WriteHeader(http.StatusCreated)
	w.Write(created.Descriptor)
}

func (s *DeploymentHandler) UpdateDeployment(w http.ResponseWriter, r *http.Request) {
	deviceId := r.PathValue("deviceId")
	deploymentId := r.PathValue("deploymentId")

	descriptor, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"deviceId":     deviceId,
			"deploymentId": deploymentId,
			"error":        err,
		}).Error("Failed to read HTTP body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	updated, err := s.svc.UpdateDeployment(r.Context(), deviceId, deploymentId, descriptor)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDeviceNotFound):
			logrus.WithFields(logrus.Fields{
				"deviceId":     deviceId,
				"deploymentId": deploymentId,
				"error":        err,
			}).Warn("Device not found")
			http.Error(w, "Device not found", http.StatusNotFound)
			return
		case errors.Is(err, domain.ErrDeploymentNotFound):
			logrus.WithFields(logrus.Fields{
				"deviceId":     deviceId,
				"deploymentId": deploymentId,
				"error":        err,
			}).Warn("Deployment not found")
			http.Error(w, "Deployment not found", http.StatusNotFound)
			return
		case errors.Is(err, domain.ErrInvalidDeploymentDescriptor):
			logrus.WithFields(logrus.Fields{
				"deviceId":     deviceId,
				"deploymentId": deploymentId,
				"error":        err,
			}).Warn("Invalid deployment descriptor")
			http.Error(w, "Invalid deployment descriptor", http.StatusBadRequest)
			return
		default:
			logrus.WithFields(logrus.Fields{
				"deviceId":     deviceId,
				"deploymentId": deploymentId,
				"error":        err,
			}).Error("Failed to update deployment")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", updated.DescriptorDigest))
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Location", fmt.Sprintf("/api/v1/devices/%s/deployments/%s/%s", deviceId, updated.Id, updated.DescriptorDigest))
	w.WriteHeader(http.StatusOK)
	w.Write(updated.Descriptor)
}

func (s *DeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	deviceId := r.PathValue("deviceId")
	deploymentId := r.PathValue("deploymentId")

	if err := s.svc.DeleteDeployment(r.Context(), deviceId, deploymentId); err != nil {
		switch {
		case errors.Is(err, domain.ErrDeviceNotFound):
			logrus.WithFields(logrus.Fields{
				"deviceId":     deviceId,
				"deploymentId": deploymentId,
				"error":        err,
			}).Warn("Device not found")
			http.Error(w, "Device not found", http.StatusNotFound)
			return
		case errors.Is(err, domain.ErrDeploymentNotFound):
			logrus.WithFields(logrus.Fields{
				"deviceId":     deviceId,
				"deploymentId": deploymentId,
				"error":        err,
			}).Warn("Deployment not found")
			http.Error(w, "Deployment not found", http.StatusNotFound)
			return
		default:
			logrus.WithFields(logrus.Fields{
				"deviceId":     deviceId,
				"deploymentId": deploymentId,
				"error":        err,
			}).Error("Failed to delete deployment")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *DeploymentHandler) GetDeploymentManifest(w http.ResponseWriter, r *http.Request) {
	deviceId := r.PathValue("deviceId")

	// Check Accept header for supported media types
	if !validateManifestAcceptHeader(r) {
		logrus.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"accept":   r.Header.Get("Accept"),
		}).Warn("Client requested unsupported media types")
		http.Error(w, "Not Acceptable: Server cannot generate a response that matches any of the media types listed in the Accept header", http.StatusNotAcceptable)
		return
	}

	manifest, err := s.svc.GetDeploymentManifest(r.Context(), deviceId)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrManifestNotFound):
			manifest = &domain.ApplicationDeploymentManifest{
				Version: 1, // empty state manifest
			}
		case errors.Is(err, domain.ErrDeviceNotFound):
			logrus.WithFields(logrus.Fields{
				"deviceId": deviceId,
				"error":    err,
			}).Warn("Device not found")
			http.Error(w, "Device not found", http.StatusNotFound)
			return
		default:
			logrus.WithFields(logrus.Fields{
				"deviceId": deviceId,
				"error":    err,
			}).Error("Failed to retrieve deployment manifest")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	response := common.GetDeploymentManifestResponse{
		ManifestVersion: manifest.Version,
		Bundle:          nil,
		Deployments:     make([]common.DeploymentDTO, len(manifest.Deployments)),
	}
	if manifest.BundleDigest != "" {
		response.Bundle = &common.BundleDTO{
			MediaType: "application/vnd.margo.bundle.v1+tar+gzip",
			Digest:    manifest.BundleDigest,
			URL:       fmt.Sprintf("/api/v1/devices/%s/bundles/%s", deviceId, manifest.BundleDigest),
		}
	}
	for i, dep := range manifest.Deployments {
		response.Deployments[i] = common.DeploymentDTO{
			DeploymentId: dep.Id,
			Digest:       dep.DescriptorDigest,
			URL:          fmt.Sprintf("/api/v1/devices/%s/deployments/%s/%s", deviceId, dep.Id, dep.DescriptorDigest),
		}
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"error":    err,
		}).Error("Failed to marshal deployment manifest response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	manifestETag := fmt.Sprintf("\"%s\"", common.CalculateDigest(jsonData))

	// Conditional request check against manifest ETag
	if clientHasETag(r.Header, manifestETag) {
		w.Header().Set("ETag", manifestETag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", manifestETag)
	w.Header().Set("Content-Type", "application/vnd.margo.manifest.v1+json")
	w.Write(jsonData)
}

func (s *DeploymentHandler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	deviceId := r.PathValue("deviceId")
	deploymentId := r.PathValue("deploymentId")
	digest := r.PathValue("digest")

	deployment, err := s.svc.GetDeployment(r.Context(), deviceId, deploymentId, digest)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDeviceNotFound):
			logrus.WithFields(logrus.Fields{"deviceId": deviceId, "error": err}).Warn("Device not found")
			http.Error(w, "Device not found", http.StatusNotFound)
			return
		case errors.Is(err, domain.ErrDeploymentNotFound):
			logrus.WithFields(logrus.Fields{"deviceId": deviceId, "deploymentId": deploymentId, "digest": digest}).Warn("Deployment not found")
			http.Error(w, "Deployment not found", http.StatusNotFound)
			return
		default:
			logrus.WithFields(logrus.Fields{"deviceId": deviceId, "deploymentId": deploymentId, "digest": digest, "error": err}).Error("Failed to retrieve deployment")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Conditional request check against deployment ETag
	deploymentETag := fmt.Sprintf("\"%s\"", deployment.DescriptorDigest)
	if clientHasETag(r.Header, deploymentETag) {
		w.Header().Set("ETag", deploymentETag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", deploymentETag)
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(deployment.Descriptor)
}

func (s *DeploymentHandler) GetBundle(w http.ResponseWriter, r *http.Request) {
	deviceId := r.PathValue("deviceId")
	digest := r.PathValue("digest")

	archive, err := s.svc.GetBundle(r.Context(), deviceId, digest)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDeviceNotFound):
			logrus.WithFields(logrus.Fields{"deviceId": deviceId, "error": err}).Warn("Device not found")
			http.Error(w, "Device not found", http.StatusNotFound)
			return
		case errors.Is(err, domain.ErrBundleNotFound):
			logrus.WithFields(logrus.Fields{"deviceId": deviceId, "digest": digest}).Warn("Bundle not found")
			http.Error(w, "Bundle not found", http.StatusNotFound)
			return
		default:
			logrus.WithFields(logrus.Fields{"deviceId": deviceId, "digest": digest, "error": err}).Error("Failed to retrieve bundle")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Conditional request check against bundle ETag
	bundleETag := fmt.Sprintf("\"%s\"", digest)
	if clientHasETag(r.Header, bundleETag) {
		w.Header().Set("ETag", bundleETag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", bundleETag)
	w.Header().Set("Content-Type", "application/vnd.margo.bundle.v1+tar+gzip")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(archive)
}

func clientHasETag(header http.Header, currentETag string) bool {
	if currentETag == "" {
		return false
	}

	values := header.Values("If-None-Match")
	if len(values) == 0 {
		return false
	}

	for _, value := range values {
		remaining := strings.TrimSpace(value)
		for len(remaining) > 0 {
			remaining = strings.TrimLeft(remaining, " \t,")
			if remaining == "" {
				break
			}
			if remaining[0] == '*' {
				return true
			}

			weak := strings.HasPrefix(remaining, "W/")
			if weak {
				remaining = strings.TrimPrefix(remaining, "W/")
				remaining = strings.TrimLeft(remaining, " \t")
			}

			if !strings.HasPrefix(remaining, "\"") {
				commaIdx := strings.IndexByte(remaining, ',')
				if commaIdx == -1 {
					break
				}
				remaining = remaining[commaIdx+1:]
				continue
			}

			end := 1
			escaped := false
			for end < len(remaining) {
				switch remaining[end] {
				case '\\':
					if escaped {
						escaped = false
					} else {
						escaped = true
					}
				case '"':
					if !escaped {
						goto capture
					}
					escaped = false
				default:
					escaped = false
				}
				end++
			}
			break

		capture:
			end++
			tag := remaining[:end]
			if weak {
				tag = "W/" + tag
			}
			if tag == currentETag {
				return true
			}
			if end >= len(remaining) {
				remaining = ""
			} else {
				remaining = remaining[end:]
			}
		}
	}

	return false
}

func validateManifestAcceptHeader(r *http.Request) bool {
	acceptHeader := r.Header.Get("Accept")
	if acceptHeader == "" {
		return true // No Accept header means accept anything
	}

	// Check if any of the supported media types are acceptable
	supportedTypes := []string{
		"application/vnd.margo.manifest.v1+json",
		"*/*",
		"application/*",
		// not implemented yet:
		// "application/vnd.margo.manifest.v1.jws+json",
	}

	for _, supportedType := range supportedTypes {
		if strings.Contains(acceptHeader, supportedType) {
			return true
		}
	}

	// No supported media type found
	return false
}
