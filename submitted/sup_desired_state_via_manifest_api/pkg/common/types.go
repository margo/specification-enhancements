package common

// Shared types between server and client.

// ApplicationDeploymentDescriptor represents the YAML structure accepted by the API
type ApplicationDeploymentDescriptor struct {
	ApiVersion string              `yaml:"apiVersion" json:"apiVersion" validate:"required"`
	Kind       string              `yaml:"kind" json:"kind" validate:"required"`
	Metadata   ApplicationMetadata `yaml:"metadata" json:"metadata" validate:"required"`
	Spec       ApplicationSpec     `yaml:"spec" json:"spec" validate:"required"`
}

type ApplicationMetadata struct {
	Annotations ApplicationAnnotations `yaml:"annotations" json:"annotations" validate:"required"`
	Name        string                 `yaml:"name" json:"name" validate:"required"`
	Namespace   string                 `yaml:"namespace" json:"namespace" validate:"required"`
}

type ApplicationAnnotations struct {
	ApplicationId string `yaml:"applicationId" json:"applicationId" validate:"required"`
	Id            string `yaml:"id" json:"id"`
}

type ApplicationSpec struct {
	DeploymentProfile DeploymentProfile           `yaml:"deploymentProfile" json:"deploymentProfile" validate:"required"`
	Parameters        map[string]ApplicationParam `yaml:"parameters" json:"parameters"`
}

type DeploymentProfile struct {
	Type       string                `yaml:"type" json:"type" validate:"required"`
	Components []DeploymentComponent `yaml:"components" json:"components" validate:"required"`
}

type DeploymentComponent struct {
	Name       string            `yaml:"name" json:"name" validate:"required"`
	Properties map[string]string `yaml:"properties" json:"properties"`
}

type ApplicationParam struct {
	Value   string                   `yaml:"value" json:"value" validate:"required"`
	Targets []ApplicationParamTarget `yaml:"targets" json:"targets" validate:"required"`
}

type ApplicationParamTarget struct {
	Pointer    string   `yaml:"pointer" json:"pointer" validate:"required"`
	Components []string `yaml:"components" json:"components"`
}

// API response DTOs (manifest and related) shared with client
type GetDeploymentManifestResponse struct {
	ManifestVersion uint64          `json:"manifestVersion"`
	Bundle          *BundleDTO      `json:"bundle"`
	Deployments     []DeploymentDTO `json:"deployments"`
}

type BundleDTO struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	URL       string `json:"url"`
}

type DeploymentDTO struct {
	DeploymentId string `json:"deploymentId"`
	Digest       string `json:"digest"`
	URL          string `json:"url"`
}
