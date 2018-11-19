package manifest

import (
	yaml "github.com/buildkite/yaml"
)

// EstafetteRelease represents a release action that in itself contains one or multiple stages
type EstafetteRelease struct {
	Name            string            `yaml:"-"`
	CloneRepository bool              `yaml:"clone,omitempty"`
	Stages          []*EstafetteStage `yaml:"-"`
}

// UnmarshalYAML customizes unmarshalling an EstafetteRelease
func (release *EstafetteRelease) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {

	var aux struct {
		Name            string        `yaml:"-"`
		CloneRepository bool          `yaml:"clone,omitempty"`
		Stages          yaml.MapSlice `yaml:"stages"`
	}

	// unmarshal to auxiliary type
	if err := unmarshal(&aux); err != nil {
		return err
	}

	// map auxiliary properties
	release.CloneRepository = aux.CloneRepository

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
		release.Stages = append(release.Stages, stage)
	}

	return nil
}

// MarshalYAML customizes marshalling an EstafetteManifest
func (release EstafetteRelease) MarshalYAML() (out interface{}, err error) {

	var aux struct {
		Name   string        `yaml:"-"`
		Stages yaml.MapSlice `yaml:"stages"`
	}

	for _, stage := range release.Stages {
		aux.Stages = append(aux.Stages, yaml.MapItem{
			Key:   stage.Name,
			Value: stage,
		})
	}

	return aux, err
}
