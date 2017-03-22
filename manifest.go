package main

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type estafetteManifest struct {
	Labels    map[string]string            `yaml:"labels,omitempty"`
	Pipelines map[string]estafettePipeline `yaml:"pipelines,omitempty"`
}

type estafettePipeline struct {
	ContainerImage   string   `yaml:"image,omitempty"`
	Shell            string   `yaml:"shell,omitempty"`
	WorkingDirectory string   `yaml:"workingDirectory,omitempty"`
	Commands         []string `yaml:"commands,omitempty"`
}

// UnmarshalYAML parses the .estafette.yaml file into an estafetteManifest object
func (c *estafetteManifest) unmarshalYAML(data []byte) error {
	return yaml.Unmarshal(data, c)
}

func readManifest(manifestPath string) (manifest estafetteManifest, err error) {

	fmt.Printf("[estafette] Reading %v file...", manifestPath)

	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return manifest, err
	}
	if err := manifest.unmarshalYAML(data); err != nil {
		return manifest, err
	}

	fmt.Printf("[estafette] Finished reading %vfile successfully", manifestPath)

	return
}
