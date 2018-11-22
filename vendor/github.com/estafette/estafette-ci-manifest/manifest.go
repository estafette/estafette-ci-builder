package manifest

import (
	"io/ioutil"
	"os"

	"github.com/rs/zerolog/log"

	yaml "gopkg.in/yaml.v2"
)

// EstafetteManifest is the object that the .estafette.yaml deserializes to
type EstafetteManifest struct {
	Builder       EstafetteBuilder    `yaml:"builder,omitempty"`
	Labels        map[string]string   `yaml:"labels,omitempty"`
	Version       EstafetteVersion    `yaml:"version,omitempty"`
	GlobalEnvVars map[string]string   `yaml:"env,omitempty"`
	Stages        []*EstafetteStage   `yaml:"-" json:"Pipelines,omitempty"`
	Releases      []*EstafetteRelease `yaml:"-"`
}

// UnmarshalYAML customizes unmarshalling an EstafetteManifest
func (c *EstafetteManifest) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {

	var aux struct {
		Builder             EstafetteBuilder  `yaml:"builder"`
		Labels              map[string]string `yaml:"labels"`
		Version             EstafetteVersion  `yaml:"version"`
		GlobalEnvVars       map[string]string `yaml:"env"`
		DeprecatedPipelines yaml.MapSlice     `yaml:"pipelines"`
		Stages              yaml.MapSlice     `yaml:"stages"`
		Releases            yaml.MapSlice     `yaml:"releases"`
	}

	// unmarshal to auxiliary type
	if err := unmarshal(&aux); err != nil {
		return err
	}

	// map auxiliary properties
	c.Builder = aux.Builder
	c.Version = aux.Version
	c.Labels = aux.Labels
	c.GlobalEnvVars = aux.GlobalEnvVars

	// provide backwards compatibility for the deprecated pipelines section now renamed to stages
	if len(aux.Stages) == 0 && len(aux.DeprecatedPipelines) > 0 {
		aux.Stages = aux.DeprecatedPipelines
	}

	for _, mi := range aux.Stages {

		bytes, err := yaml.Marshal(mi.Value)
		if err != nil {
			return err
		}

		var stage *EstafetteStage
		if err := yaml.Unmarshal(bytes, &stage); err != nil {
			return err
		}
		if stage == nil {
			stage = &EstafetteStage{}
		}

		stage.Name = mi.Key.(string)
		stage.setDefaults()
		c.Stages = append(c.Stages, stage)
	}

	for _, mi := range aux.Releases {

		bytes, err := yaml.Marshal(mi.Value)
		if err != nil {
			return err
		}

		var release *EstafetteRelease
		if err := yaml.Unmarshal(bytes, &release); err != nil {
			return err
		}
		if release == nil {
			release = &EstafetteRelease{}
		}

		release.Name = mi.Key.(string)
		c.Releases = append(c.Releases, release)
	}

	// set default property values
	c.setDefaults()

	return nil
}

// MarshalYAML customizes marshalling an EstafetteManifest
func (c EstafetteManifest) MarshalYAML() (out interface{}, err error) {
	var aux struct {
		Builder       EstafetteBuilder  `yaml:"builder,omitempty"`
		Labels        map[string]string `yaml:"labels,omitempty"`
		Version       EstafetteVersion  `yaml:"version,omitempty"`
		GlobalEnvVars map[string]string `yaml:"env,omitempty"`
		Stages        yaml.MapSlice     `yaml:"stages,omitempty"`
		Releases      yaml.MapSlice     `yaml:"releases,omitempty"`
	}

	aux.Builder = c.Builder
	aux.Labels = c.Labels
	aux.Version = c.Version
	aux.GlobalEnvVars = c.GlobalEnvVars

	for _, stage := range c.Stages {
		aux.Stages = append(aux.Stages, yaml.MapItem{
			Key:   stage.Name,
			Value: stage,
		})
	}
	for _, release := range c.Releases {
		aux.Releases = append(aux.Releases, yaml.MapItem{
			Key:   release.Name,
			Value: release,
		})
	}

	return aux, err
}

// setDefaults sets default values for properties of EstafetteManifest if not defined
func (c *EstafetteManifest) setDefaults() {
	c.Builder.setDefaults()
	c.Version.setDefaults()
}

// Exists checks whether the .estafette.yaml exists
func Exists(manifestPath string) bool {

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// does not exist
		return false
	}

	// does exist
	return true
}

// ReadManifestFromFile reads the .estafette.yaml into an EstafetteManifest object
func ReadManifestFromFile(manifestPath string) (manifest EstafetteManifest, err error) {

	log.Info().Msgf("Reading %v file...", manifestPath)

	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return manifest, err
	}

	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	manifest.setDefaults()

	log.Info().Msgf("Finished reading %v file successfully", manifestPath)

	return
}

// ReadManifest reads the string representation of .estafette.yaml into an EstafetteManifest object
func ReadManifest(manifestString string) (manifest EstafetteManifest, err error) {

	log.Info().Msg("Reading manifest from string...")

	if err := yaml.Unmarshal([]byte(manifestString), &manifest); err != nil {
		return manifest, err
	}
	manifest.setDefaults()

	log.Info().Msg("Finished unmarshalling manifest from string successfully")

	return
}
