package contracts

import (
	"github.com/estafette/estafette-ci-manifest"
)

// ReleaseTarget contains the information to visualize and trigger release
type ReleaseTarget struct {
	Name           string                             `yaml:"name"`
	Actions        []*manifest.EstafetteReleaseAction `yaml:"actions,omitempty"`
	ActiveReleases []*Release                         `yaml:"activeReleases,omitempty"`
}
