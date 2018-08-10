package manifest

import (
	yaml "gopkg.in/yaml.v2"
)

// EstafetteRelease represents a release action that in itself contains one or multiple stages
type EstafetteRelease struct {
	Name               string            `yaml:"-"`
	CloneGitRepository bool              `yaml:"-"`
	Stages             []*EstafetteStage `yaml:"-"`
}

// UnmarshalYAML customizes unmarshalling an EstafetteRelease
func (release *EstafetteRelease) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {

	var aux yaml.MapSlice
	var clone bool

	// unmarshal to auxiliary type
	if err := unmarshal(&aux); err != nil {
		return err
	}

	// map auxiliary properties
	for _, mi := range aux {

		bytes, err := yaml.Marshal(mi.Value)
		if err != nil {
			return err
		}

		if mi.Key == "clone" {
			if err := yaml.Unmarshal(bytes, &clone); err != nil {
				return err
			}
			release.CloneGitRepository = clone
			continue
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
	var aux yaml.MapSlice

	aux = append(aux, yaml.MapItem{
		Key:   "clone",
		Value: release.CloneGitRepository,
	})

	for _, stage := range release.Stages {
		aux = append(aux, yaml.MapItem{
			Key:   stage.Name,
			Value: stage,
		})
	}

	return aux, err
}
