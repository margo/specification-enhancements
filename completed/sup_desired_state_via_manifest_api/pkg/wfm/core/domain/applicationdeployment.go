package domain

type ApplicationDeploymentManifest struct {
	Version       uint64
	BundleArchive []byte
	BundleDigest  string
	Deployments   []ApplicationDeployment
}

type ApplicationDeployment struct {
	Id               string
	Descriptor       []byte
	DescriptorDigest string
}
