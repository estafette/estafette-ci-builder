package contracts

import (
	"time"

	manifest "github.com/estafette/estafette-ci-manifest"
)

// Build represents a specific build, including version number, repo, branch, revision, labels and manifest
type Build struct {
	ID                   string                      `json:"id"`
	RepoSource           string                      `json:"repoSource"`
	RepoOwner            string                      `json:"repoOwner"`
	RepoName             string                      `json:"repoName"`
	RepoBranch           string                      `json:"repoBranch"`
	RepoRevision         string                      `json:"repoRevision"`
	BuildVersion         string                      `json:"buildVersion,omitempty"`
	BuildStatus          string                      `json:"buildStatus,omitempty"`
	Labels               []Label                     `json:"labels,omitempty"`
	ReleaseTargets       []ReleaseTarget             `json:"releaseTargets,omitempty"`
	Manifest             string                      `json:"manifest,omitempty"`
	ManifestWithDefaults string                      `json:"manifestWithDefaults,omitempty"`
	Commits              []GitCommit                 `json:"commits,omitempty"`
	Triggers             []manifest.EstafetteTrigger `json:"triggers,omitempty"`
	InsertedAt           time.Time                   `json:"insertedAt"`
	UpdatedAt            time.Time                   `json:"updatedAt"`
	Duration             time.Duration               `json:"duration"`
	ManifestObject       *manifest.EstafetteManifest `json:"-"`
}
