package contracts

import "time"

// Pipeline represents a pipeline with the latest build info, including version number, repo, branch, revision, labels and manifest
type Pipeline struct {
	ID           string      `json:"id"`
	RepoSource   string      `json:"repoSource"`
	RepoOwner    string      `json:"repoOwner"`
	RepoName     string      `json:"repoName"`
	RepoBranch   string      `json:"repoBranch"`
	RepoRevision string      `json:"repoRevision"`
	BuildVersion string      `json:"buildVersion"`
	BuildStatus  string      `json:"buildStatus"`
	Labels       []Label     `json:"labels"`
	Manifest     string      `json:"manifest"`
	Commits      []GitCommit `json:"commits"`
	InsertedAt   time.Time   `json:"insertedAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}
