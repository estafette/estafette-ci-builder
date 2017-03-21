package main

import yaml "gopkg.in/yaml.v2"

type estafetteManifest struct {
	Labels    map[string]string            `yaml:"labels,omitempty"`
	Pipelines map[string]estafettePipeline `yaml:"pipelines,omitempty"`
}

type estafettePipeline struct {
	ContainerImage string   `yaml:"image,omitempty"`
	Shell          string   `yaml:"shell,omitempty"`
	Commands       []string `yaml:"commands,omitempty"`
}

// UnmarshalYAML parses the .estafette.yaml file into an estafetteManifest object
func (c *estafetteManifest) UnmarshalYAML(data []byte) error {
	return yaml.Unmarshal(data, c)
}
