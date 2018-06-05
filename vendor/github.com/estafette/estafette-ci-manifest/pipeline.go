package manifest

// EstafettePipeline is the object that parts of the .estafette.yaml deserialize to
type EstafettePipeline struct {
	Name             string
	ContainerImage   string            `yaml:"image,omitempty"`
	Shell            string            `yaml:"shell,omitempty"`
	WorkingDirectory string            `yaml:"workDir,omitempty"`
	Commands         []string          `yaml:"commands,omitempty"`
	When             string            `yaml:"when,omitempty"`
	EnvVars          map[string]string `yaml:"env,omitempty"`
	AutoInjected     bool              `yaml:"autoInjected,omitempty"`
	CustomProperties map[string]interface{}
}
