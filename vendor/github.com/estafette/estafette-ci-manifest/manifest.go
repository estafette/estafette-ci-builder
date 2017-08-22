package manifest

import (
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"github.com/rs/zerolog/log"

	yaml "gopkg.in/yaml.v2"
)

// EstafetteManifest is the object that the .estafette.yaml deserializes to
type EstafetteManifest struct {
	Builder   EstafetteBuilder     `yaml:"builder,omitempty"`
	Labels    map[string]string    `yaml:"labels,omitempty"`
	Pipelines []*EstafettePipeline `yaml:"dummy,omitempty"`
}

// EstafettePipeline is the object that parts of the .estafette.yaml deserialize to
type EstafettePipeline struct {
	Name             string
	ContainerImage   string            `yaml:"image,omitempty"`
	Shell            string            `yaml:"shell,omitempty"`
	WorkingDirectory string            `yaml:"workDir,omitempty"`
	Commands         []string          `yaml:"commands,omitempty"`
	When             string            `yaml:"when,omitempty"`
	EnvVars          map[string]string `yaml:"env,omitempty"`
	CustomProperties map[string]interface{}
}

// EstafetteBuilder contains configuration for the ci-builder component
type EstafetteBuilder struct {
	Track string `yaml:"track,omitempty"`
}

// unmarshalYAML parses the .estafette.yaml file into an EstafetteManifest object
func (c *EstafetteManifest) unmarshalYAML(data []byte) error {

	err := yaml.Unmarshal(data, c)
	if err != nil {
		log.Error().Err(err).Msg("Unmarshalling .estafette.yaml manifest failed")
		return err
	}

	// set default for Builder.Track if not set
	if c.Builder.Track == "" {
		c.Builder.Track = "stable"
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
				p := EstafettePipeline{}
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
				p.CustomProperties = map[string]interface{}{}
				propertiesMap := map[string]interface{}{}
				err = yaml.Unmarshal(out, &propertiesMap)
				if err != nil {
					return err
				}
				if propertiesMap != nil && len(propertiesMap) > 0 {
					for k, v := range propertiesMap {
						if !isReservedPopertyName(reservedPropertyNames, k) {
							p.CustomProperties[k] = v
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
	val := reflect.ValueOf(EstafettePipeline{})
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
	if err := manifest.unmarshalYAML(data); err != nil {
		return manifest, err
	}

	log.Info().Msgf("Finished reading %v file successfully", manifestPath)

	return
}

// ReadManifest reads the string representation of .estafette.yaml into an EstafetteManifest object
func ReadManifest(manifestString string) (manifest EstafetteManifest, err error) {

	log.Info().Msg("Reading manifest from string...")

	if err := manifest.unmarshalYAML([]byte(manifestString)); err != nil {
		return manifest, err
	}

	log.Info().Msg("Finished unmarshalling manifest from string successfully")

	return
}
