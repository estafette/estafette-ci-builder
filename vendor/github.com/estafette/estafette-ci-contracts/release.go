package contracts

import "time"

// Release represents a release of a pipeline
type Release struct {
	Name           string     `json:"name"`
	ID             int        `json:"id,omitempty"`
	RepoSource     string     `json:"repoSource,omitempty"`
	RepoOwner      string     `json:"repoOwner,omitempty"`
	RepoName       string     `json:"repoName,omitempty"`
	ReleaseVersion string     `json:"releaseVersion,omitempty"`
	ReleaseStatus  string     `json:"releaseStatus,omitempty"`
	TriggeredBy    string     `json:"triggeredBy,omitempty"`
	InsertedAt     *time.Time `json:"insertedAt,omitempty"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
}
