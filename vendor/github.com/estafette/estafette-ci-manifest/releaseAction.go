package manifest

// EstafetteReleaseAction represents an action on a release target that controls what happens by running the release stage
type EstafetteReleaseAction struct {
	Name string `yaml:"name"`
}
