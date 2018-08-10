package contracts

import "time"

// ReleaseLog represents a release log for a specific release
type ReleaseLog struct {
	ID         string         `json:"id,omitempty"`
	RepoSource string         `json:"repoSource"`
	RepoOwner  string         `json:"repoOwner"`
	RepoName   string         `json:"repoName"`
	ReleaseID  string         `json:"releaseID"`
	Steps      []BuildLogStep `json:"steps"`
	InsertedAt time.Time      `json:"insertedAt"`
}
