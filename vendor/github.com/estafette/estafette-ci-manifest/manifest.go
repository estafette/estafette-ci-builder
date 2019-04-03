package manifest

import (
	"fmt"
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
	Triggers      []*EstafetteTrigger `yaml:"triggers,omitempty"`
	Stages        []*EstafetteStage   `yaml:"-" json:"Pipelines,omitempty"`
	Releases      []*EstafetteRelease `yaml:"-"`
}

// UnmarshalYAML customizes unmarshalling an EstafetteManifest
func (c *EstafetteManifest) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {

	var aux struct {
		Builder             EstafetteBuilder    `yaml:"builder"`
		Labels              map[string]string   `yaml:"labels"`
		Version             EstafetteVersion    `yaml:"version"`
		GlobalEnvVars       map[string]string   `yaml:"env"`
		DeprecatedPipelines yaml.MapSlice       `yaml:"pipelines"`
		Triggers            []*EstafetteTrigger `yaml:"triggers"`
		Stages              yaml.MapSlice       `yaml:"stages"`
		Releases            yaml.MapSlice       `yaml:"releases"`
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
	c.Triggers = aux.Triggers

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
		Builder       EstafetteBuilder    `yaml:"builder,omitempty"`
		Labels        map[string]string   `yaml:"labels,omitempty"`
		Version       EstafetteVersion    `yaml:"version,omitempty"`
		GlobalEnvVars map[string]string   `yaml:"env,omitempty"`
		Triggers      []*EstafetteTrigger `yaml:"triggers,omitempty"`
		Stages        yaml.MapSlice       `yaml:"stages,omitempty"`
		Releases      yaml.MapSlice       `yaml:"releases,omitempty"`
	}

	aux.Builder = c.Builder
	aux.Labels = c.Labels
	aux.Version = c.Version
	aux.GlobalEnvVars = c.GlobalEnvVars
	aux.Triggers = c.Triggers

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

	for _, t := range c.Triggers {
		t.SetDefaults("build", "")
	}

	for _, r := range c.Releases {
		for _, t := range r.Triggers {
			t.SetDefaults("release", r.Name)
		}
	}
}

// Validate checks if the manifest is valid
func (c *EstafetteManifest) Validate() (err error) {

	if len(c.Stages) == 0 {
		return fmt.Errorf("The manifest should define 1 or more stages")
	}

	for _, t := range c.Triggers {
		err = t.Validate("build", "")
		if err != nil {
			return
		}
	}

	for _, r := range c.Releases {
		for _, t := range r.Triggers {
			err = t.Validate("release", r.Name)
			if err != nil {
				return
			}
		}
	}

	return nil
}

// GetAllTriggers returns both build and release triggers as one list
func (c *EstafetteManifest) GetAllTriggers() []EstafetteTrigger {
	// collect both build and release triggers
	triggers := make([]EstafetteTrigger, 0)

	// add all build triggers
	for _, t := range c.Triggers {
		if t != nil {
			triggers = append(triggers, *t)
		}
	}

	// add all release triggers
	for _, r := range c.Releases {
		for _, t := range r.Triggers {
			if t != nil {
				triggers = append(triggers, *t)
			}
		}
	}

	return triggers
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

	// unmarshal strict, so non-defined properties or incorrect nesting will fail
	if err := yaml.UnmarshalStrict(data, &manifest); err != nil {
		return manifest, err
	}

	// set defaults
	manifest.setDefaults()

	// check if manifest is valid
	err = manifest.Validate()
	if err != nil {
		return manifest, err
	}

	log.Info().Msgf("Finished reading %v file successfully", manifestPath)

	return
}

// ReadManifest reads the string representation of .estafette.yaml into an EstafetteManifest object
func ReadManifest(manifestString string) (manifest EstafetteManifest, err error) {

	log.Info().Msg("Reading manifest from string...")

	// unmarshal strict, so non-defined properties or incorrect nesting will fail
	if err := yaml.UnmarshalStrict([]byte(manifestString), &manifest); err != nil {
		return manifest, err
	}

	// set defaults
	manifest.setDefaults()

	// check if manifest is valid
	err = manifest.Validate()
	if err != nil {
		return manifest, err
	}

	log.Info().Msg("Finished unmarshalling manifest from string successfully")

	return
}
