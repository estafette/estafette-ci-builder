package manifest

// EstafetteStage represents a stage of a build pipeline or release
type EstafetteStage struct {
	Name             string                 `yaml:"-"`
	ContainerImage   string                 `yaml:"image,omitempty"`
	Shell            string                 `yaml:"shell,omitempty"`
	WorkingDirectory string                 `yaml:"workDir,omitempty"`
	Commands         []string               `yaml:"commands,omitempty"`
	When             string                 `yaml:"when,omitempty"`
	EnvVars          map[string]string      `yaml:"env,omitempty"`
	AutoInjected     bool                   `yaml:"autoInjected,omitempty"`
	Retries          int                    `yaml:"retries,omitempty"`
	CustomProperties map[string]interface{} `yaml:",inline"`
}

// UnmarshalYAML customizes unmarshalling an EstafetteStage
func (stage *EstafetteStage) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {

	var aux struct {
		Name             string                 `yaml:"-"`
		ContainerImage   string                 `yaml:"image"`
		Shell            string                 `yaml:"shell"`
		WorkingDirectory string                 `yaml:"workDir"`
		Commands         []string               `yaml:"commands"`
		When             string                 `yaml:"when"`
		EnvVars          map[string]string      `yaml:"env"`
		AutoInjected     bool                   `yaml:"autoInjected"`
		Retries          int                    `yaml:"retries,omitempty"`
		CustomProperties map[string]interface{} `yaml:",inline"`
	}

	// unmarshal to auxiliary type
	if err := unmarshal(&aux); err != nil {
		return err
	}

	// map auxiliary properties
	stage.ContainerImage = aux.ContainerImage
	stage.Shell = aux.Shell
	stage.WorkingDirectory = aux.WorkingDirectory
	stage.Commands = aux.Commands
	stage.When = aux.When
	stage.EnvVars = aux.EnvVars
	stage.AutoInjected = aux.AutoInjected
	stage.Retries = aux.Retries

	// fix for map[interface{}]interface breaking json.marshal - see https://github.com/go-yaml/yaml/issues/139
	stage.CustomProperties = cleanUpStringMap(aux.CustomProperties)

	// set default property values
	stage.setDefaults()

	return nil
}

// setDefaults sets default values for properties of EstafetteStage if not defined
func (stage *EstafetteStage) setDefaults() {
	// set default for Shell if not set
	if stage.Shell == "" {
		stage.Shell = "/bin/sh"
	}

	// set default for WorkingDirectory if not set
	if stage.WorkingDirectory == "" {
		stage.WorkingDirectory = "/estafette-work"
	}

	// set default for When if not set
	if stage.When == "" {
		stage.When = "status == 'succeeded'"
	}
}
