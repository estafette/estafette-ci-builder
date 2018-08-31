package contracts

// ContainerRepositoryCredentialConfig is used to authenticate for (private) container repositories
type ContainerRepositoryCredentialConfig struct {
	Repository string `yaml:"repository"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}
