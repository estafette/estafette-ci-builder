package contracts

import "strings"

// ContainerRepositoryCredentialConfig is used to authenticate for (private) container repositories (will be replaced by CredentialConfig eventually)
type ContainerRepositoryCredentialConfig struct {
	Repository string `yaml:"repository"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}

// BuilderConfig parameterizes a build/release job
type BuilderConfig struct {
	Action       *string             `json:"action,omitempty"`
	Track        *string             `json:"track,omitempty"`
	Git          *GitConfig          `json:"git,omitempty"`
	BuildVersion *BuildVersionConfig `json:"buildVersion,omitempty"`

	Credentials   []*CredentialConfig   `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	TrustedImages []*TrustedImageConfig `yaml:"trustedImages,omitempty" json:"trustedImages,omitempty"`
}

// CredentialConfig is used to store credentials for every type of authenticated service you can use from docker registries, to kubernetes engine to, github apis, bitbucket;
// in combination with trusted images access to these centrally stored credentials can be limited
type CredentialConfig struct {
	Name                 string                 `yaml:"name" json:"name"`
	Type                 string                 `yaml:"type" json:"type"`
	AdditionalProperties map[string]interface{} `yaml:",inline" json:"additionalProperties,omitempty"`
}

// TrustedImageConfig allows trusted images to run docker commands or receive specific credentials
type TrustedImageConfig struct {
	ImagePath               string   `yaml:"path" json:"path"`
	RunDocker               bool     `yaml:"runDocker" json:"runDocker"`
	InjectedCredentialTypes []string `yaml:"injectedCredentialTypes" json:"injectedCredentialTypes"`
}

// GitConfig contains all information for cloning the git repository for building/releasing a specific version
type GitConfig struct {
	RepoSource   string `json:"repoSource"`
	RepoOwner    string `json:"repoOwner"`
	RepoName     string `json:"repoName"`
	RepoBranch   string `json:"repoBranch"`
	RepoRevision string `json:"repoRevision"`
}

// BuildVersionConfig contains all information regarding the version number to build or release
type BuildVersionConfig struct {
	Version       string  `json:"version"`
	Major         *int    `json:"major,omitempty"`
	Minor         *int    `json:"minor,omitempty"`
	Patch         *string `json:"patch,omitempty"`
	AutoIncrement *int    `json:"autoincrement,omitempty"`
}

// GetCredentialsByType returns all credentials of a certain type
func (c *BuilderConfig) GetCredentialsByType(filterType string) []*CredentialConfig {

	filteredCredentials := []*CredentialConfig{}

	for _, cred := range c.Credentials {
		if cred.Type == filterType {
			filteredCredentials = append(filteredCredentials, cred)
		}
	}

	return filteredCredentials
}

// GetCredentialsForTrustedImage returns all credentials of a certain type
func (c *BuilderConfig) GetCredentialsForTrustedImage(trustedImage TrustedImageConfig) []*CredentialConfig {

	credentialsForTrustedImage := []*CredentialConfig{}

	for _, filterType := range trustedImage.InjectedCredentialTypes {
		credentialsForTrustedImage = append(credentialsForTrustedImage, c.GetCredentialsByType(filterType)...)
	}

	return credentialsForTrustedImage
}

// GetTrustedImage returns a trusted image if the path without tag matches any of the trustedImages
func (c *BuilderConfig) GetTrustedImage(imagePath string) *TrustedImageConfig {

	imagePathSlice := strings.Split(imagePath, ":")
	imagePathWithoutTag := imagePathSlice[0]

	for _, trustedImage := range c.TrustedImages {
		if trustedImage.ImagePath == imagePathWithoutTag {
			return trustedImage
		}
	}

	return nil
}
