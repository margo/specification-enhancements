package domain

import (
	"errors"
)

var (
	ErrInternal                    = errors.New("internal error")
	ErrInvalidDeploymentDescriptor = errors.New("invalid ApplicationDeployment descriptor")
	ErrDeviceNotFound              = errors.New("device not found")
	ErrManifestNotFound            = errors.New("application deployment manifest not found")
	ErrDeploymentNotFound          = errors.New("application deployment not found")
	ErrBundleNotFound              = errors.New("application deployment bundle not found")
)
