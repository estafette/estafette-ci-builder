package main

import (
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"github.com/rs/zerolog/log"

	yaml "gopkg.in/yaml.v2"
)

type estafetteManifest struct {
	Labels    map[string]string    `yaml:"labels,omitempty"`
	Pipelines []*estafettePipeline `yaml:"dummy,omitempty"`
}

type estafettePipeline struct {
	Name             string
	ContainerImage   string            `yaml:"image,omitempty"`
	Shell            string            `yaml:"shell,omitempty"`
	WorkingDirectory string            `yaml:"workDir,omitempty"`
	Commands         []string          `yaml:"commands,omitempty"`
	When             string            `yaml:"when,omitempty"`
	EnvVars          map[string]string `yaml:"env,omitempty"`
	CustomProperties map[string]string
}

// UnmarshalYAML parses the .estafette.yaml file into an estafetteManifest object
func (c *estafetteManifest) unmarshalYAML(data []byte) error {

	err := yaml.Unmarshal(data, c)
	if err != nil {
		log.Error().Err(err).Msg("Unmarshalling .estafette.yaml manifest failed")
		return err
	}

	// create list of reserved property names
	reservedPropertyNames := getReservedPropertyNames()

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

				// assign all unknown (non-reserved) properties to CustomProperties
				p.CustomProperties = map[string]string{}
				propertiesMap := map[string]interface{}{}
				err = yaml.Unmarshal(out, &propertiesMap)
				if err != nil {
					return err
				}
				if propertiesMap != nil && len(propertiesMap) > 0 {
					for k, v := range propertiesMap {
						if !isReservedPopertyName(reservedPropertyNames, k) {
							if s, isString := v.(string); isString {
								p.CustomProperties[k] = s
							}
						}
					}
				}

				// add pipeline
				c.Pipelines = append(c.Pipelines, &p)
			}
		}
	}

	return nil
}

func getReservedPropertyNames() (names []string) {
	// create list of reserved property names
	reservedPropertyNames := []string{}
	val := reflect.ValueOf(estafettePipeline{})
	for i := 0; i < val.Type().NumField(); i++ {
		yamlName := val.Type().Field(i).Tag.Get("yaml")
		if yamlName != "" {
			reservedPropertyNames = append(reservedPropertyNames, strings.Replace(yamlName, ",omitempty", "", 1))
		}
		propertyName := val.Type().Field(i).Name
		if propertyName != "" {
			reservedPropertyNames = append(reservedPropertyNames, propertyName)
		}
	}

	return reservedPropertyNames
}

func isReservedPopertyName(s []string, e string) bool {

	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func manifestExists(manifestPath string) bool {

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// does not exist
		return false
	}

	// does exist
	return true
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
