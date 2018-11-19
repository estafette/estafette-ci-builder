package contracts

import (
	"github.com/estafette/estafette-ci-manifest"
)

// ReleaseTarget contains the information to visualize and trigger release
type ReleaseTarget struct {
	Name           string                            `json:"name"`
	Actions        []manifest.EstafetteReleaseAction `json:"actions,omitempty"`
	ActiveReleases []Release                         `json:"activeReleases,omitempty"`
}
