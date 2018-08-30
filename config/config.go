package config

// PrivateContainerRegistryConfig is used to authenticate for private container registries
type PrivateContainerRegistryConfig struct {
	Server   string `yaml:"server"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
