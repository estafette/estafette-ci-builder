package main

import (
	"io/ioutil"

	"github.com/rs/zerolog/log"

	yaml "gopkg.in/yaml.v2"
)

type estafetteManifest struct {
	Labels    map[string]string    `yaml:"labels,omitempty"`
	Pipelines []*estafettePipeline `yaml:"dummy,omitempty"`
}

type estafettePipeline struct {
	Name             string
	ContainerImage   string   `yaml:"image,omitempty"`
	Shell            string   `yaml:"shell,omitempty"`
	WorkingDirectory string   `yaml:"workDir,omitempty"`
	Commands         []string `yaml:"commands,omitempty"`
	When             string   `yaml:"when,omitempty"`
}

// UnmarshalYAML parses the .estafette.yaml file into an estafetteManifest object
func (c *estafetteManifest) unmarshalYAML(data []byte) error {

	err := yaml.Unmarshal(data, c)
	if err != nil {
		log.Error().Err(err).Msg("Unmarshalling .estafette.yaml manifest failed")
		return err
	}

	// to preserve order for the pipelines use MapSlice
	outerSlice := yaml.MapSlice{}
	err = yaml.Unmarshal(data, &outerSlice)
	if err != nil {
		return err
	}

	for _, s := range outerSlice {

		if s.Key == "pipelines" {

			// map value back to yaml in order to unmarshal again
			out, err := yaml.Marshal(s.Value)
			if err != nil {
				return err
			}

			// unmarshal again into map slice
			innerSlice := yaml.MapSlice{}
			err = yaml.Unmarshal(out, &innerSlice)
			if err != nil {
				return err
			}

			for _, t := range innerSlice {

				// map value back to yaml in order to unmarshal again
				out, err := yaml.Marshal(t.Value)
				if err != nil {
					return err
				}

				// unmarshal again into estafettePipeline
				p := estafettePipeline{}
				err = yaml.Unmarshal(out, &p)
				if err != nil {
					return err
				}

				// set estafettePipeline name
				p.Name = t.Key.(string)

				// set default for Shell if not set
				if p.Shell == "" {
					p.Shell = "/bin/sh"
				}

				// set default for WorkingDirectory if not set
				if p.WorkingDirectory == "" {
					p.WorkingDirectory = "/estafette-work"
				}

				// set default for When if not set
				if p.When == "" {
					p.When = "status == 'succeeded'"
				}

				// add pipeline
				c.Pipelines = append(c.Pipelines, &p)
			}
		}
	}

	return nil
}

func readManifest(manifestPath string) (manifest estafetteManifest, err error) {

	log.Info().Msgf("Reading %v file...", manifestPath)

	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return manifest, err
	}
	if err := manifest.unmarshalYAML(data); err != nil {
		return manifest, err
	}

	log.Info().Msgf("Finished reading %v file successfully", manifestPath)

	return
}
