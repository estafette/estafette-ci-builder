package contracts

import (
	"strings"

	"github.com/estafette/estafette-ci-manifest"
)

// ContainerRepositoryCredentialConfig is used to authenticate for (private) container repositories (will be replaced by CredentialConfig eventually)
type ContainerRepositoryCredentialConfig struct {
	Repository string `yaml:"repository"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}

// BuilderConfig parameterizes a build/release job
type BuilderConfig struct {
	Action         *string `json:"action,omitempty"`
	Track          *string `json:"track,omitempty"`
	RegistryMirror *string `json:"registryMirror,omitempty"`

	Manifest *manifest.EstafetteManifest `json:"manifest,omitempty"`

	JobName     *string `json:"jobName,omitempty"`
	ReleaseName *string `json:"releaseName,omitempty"`

	CIServer      *CIServerConfig      `json:"ciServer,omitempty"`
	BuildParams   *BuildParamsConfig   `json:"buildParams,omitempty"`
	ReleaseParams *ReleaseParamsConfig `json:"releaseParams,omitempty"`

	Git           *GitConfig            `json:"git,omitempty"`
	BuildVersion  *BuildVersionConfig   `json:"buildVersion,omitempty"`
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

// UnmarshalYAML customizes unmarshalling an EstafetteStage
func (cc *CredentialConfig) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {

	var aux struct {
		Name                 string                 `yaml:"name" json:"name"`
		Type                 string                 `yaml:"type" json:"type"`
		AdditionalProperties map[string]interface{} `yaml:",inline" json:"additionalProperties,omitempty"`
	}

	// unmarshal to auxiliary type
	if err := unmarshal(&aux); err != nil {
		return err
	}

	// map auxiliary properties
	cc.Name = aux.Name
	cc.Type = aux.Type

	// fix for map[interface{}]interface breaking json.marshal - see https://github.com/go-yaml/yaml/issues/139
	cc.AdditionalProperties = cleanUpStringMap(aux.AdditionalProperties)

	return nil
}

// TrustedImageConfig allows trusted images to run docker commands or receive specific credentials
type TrustedImageConfig struct {
	ImagePath               string   `yaml:"path" json:"path"`
	RunPrivileged           bool     `yaml:"runPrivileged" json:"runPrivileged"`
	RunDocker               bool     `yaml:"runDocker" json:"runDocker"`
	AllowCommands           bool     `yaml:"allowCommands" json:"allowCommands"`
	InjectedCredentialTypes []string `yaml:"injectedCredentialTypes,omitempty" json:"injectedCredentialTypes,omitempty"`
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
	Label         *string `json:"label,omitempty"`
	AutoIncrement *int    `json:"autoincrement,omitempty"`
}

// CIServerConfig has a number of config items related to communication or linking to the CI server
type CIServerConfig struct {
	BaseURL          string `json:"baseUrl"`
	BuilderEventsURL string `json:"builderEventsUrl"`
	PostLogsURL      string `json:"postLogsUrl"`
	APIKey           string `json:"apiKey"`
}

// BuildParamsConfig has config specific to builds
type BuildParamsConfig struct {
	BuildID int `json:"buildID"`
}

// ReleaseParamsConfig has config specific to releases
type ReleaseParamsConfig struct {
	ReleaseName   string `json:"releaseName"`
	ReleaseID     int    `json:"releaseID"`
	ReleaseAction string `json:"releaseAction,omitempty"`
	TriggeredBy   string `json:"triggeredBy,omitempty"`
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

// GetCredentialsByType returns all credentials of a certain type
func GetCredentialsByType(credentials []*CredentialConfig, filterType string) []*CredentialConfig {

	filteredCredentials := []*CredentialConfig{}

	for _, cred := range credentials {
		if cred.Type == filterType {
			filteredCredentials = append(filteredCredentials, cred)
		}
	}

	return filteredCredentials
}

// GetCredentialsForTrustedImage returns all credentials of a certain type
func (c *BuilderConfig) GetCredentialsForTrustedImage(trustedImage TrustedImageConfig) map[string][]*CredentialConfig {
	return GetCredentialsForTrustedImage(c.Credentials, trustedImage)
}

// GetCredentialsForTrustedImage returns all credentials of a certain type
func GetCredentialsForTrustedImage(credentials []*CredentialConfig, trustedImage TrustedImageConfig) map[string][]*CredentialConfig {

	credentialMap := map[string][]*CredentialConfig{}

	for _, filterType := range trustedImage.InjectedCredentialTypes {
		credsByType := GetCredentialsByType(credentials, filterType)
		if len(credsByType) > 0 {
			credentialMap[filterType] = credsByType
		}
	}

	return credentialMap
}

// GetTrustedImage returns a trusted image if the path without tag matches any of the trustedImages
func (c *BuilderConfig) GetTrustedImage(imagePath string) *TrustedImageConfig {
	return GetTrustedImage(c.TrustedImages, imagePath)
}

// GetTrustedImage returns a trusted image if the path without tag matches any of the trustedImages
func GetTrustedImage(trustedImages []*TrustedImageConfig, imagePath string) *TrustedImageConfig {

	imagePathSlice := strings.Split(imagePath, ":")
	imagePathWithoutTag := imagePathSlice[0]

	for _, trustedImage := range trustedImages {
		if trustedImage.ImagePath == imagePathWithoutTag {
			return trustedImage
		}
	}

	return nil
}

// FilterTrustedImages returns only trusted images used in the stages
func FilterTrustedImages(trustedImages []*TrustedImageConfig, stages []*manifest.EstafetteStage) []*TrustedImageConfig {

	filteredImages := []*TrustedImageConfig{}

	for _, s := range stages {
		ti := GetTrustedImage(trustedImages, s.ContainerImage)
		if ti != nil {
			alreadyAdded := false
			for _, fi := range filteredImages {
				if fi.ImagePath == ti.ImagePath {
					alreadyAdded = true
					break
				}
			}

			if !alreadyAdded {
				filteredImages = append(filteredImages, ti)
			}
		}
	}

	return filteredImages
}

// FilterCredentials returns only credentials used by the trusted images
func FilterCredentials(credentials []*CredentialConfig, trustedImages []*TrustedImageConfig) []*CredentialConfig {

	filteredCredentials := []*CredentialConfig{}

	for _, i := range trustedImages {
		credMap := GetCredentialsForTrustedImage(credentials, *i)

		// loop all items in credmap and add to filtered credentials if they haven't been already added
		for _, v := range credMap {
			filteredCredentials = AddCredentialsIfNotPresent(filteredCredentials, v)
		}
	}

	return filteredCredentials
}

// AddCredentialsIfNotPresent adds new credentials to source credentials if they're not present yet
func AddCredentialsIfNotPresent(sourceCredentials []*CredentialConfig, newCredentials []*CredentialConfig) []*CredentialConfig {

	for _, c := range newCredentials {

		alreadyAdded := false
		for _, fc := range sourceCredentials {
			if fc.Name == c.Name && fc.Type == c.Type {
				alreadyAdded = true
				break
			}
		}

		if !alreadyAdded {
			sourceCredentials = append(sourceCredentials, c)
		}
	}

	return sourceCredentials
}
