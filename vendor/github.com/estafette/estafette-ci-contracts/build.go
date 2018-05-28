package contracts

import "time"

// Build represents a specific build, including version number, repo, branch, revision, labels and manifest
type Build struct {
	ID           string    `jsonapi:"primary,builds"`
	RepoSource   string    `jsonapi:"attr,repo-source"`
	RepoOwner    string    `jsonapi:"attr,repo-owner"`
	RepoName     string    `jsonapi:"attr,repo-name"`
	RepoBranch   string    `jsonapi:"attr,repo-branch"`
	RepoRevision string    `jsonapi:"attr,repo-revision"`
	BuildVersion string    `jsonapi:"attr,build-version"`
	BuildStatus  string    `jsonapi:"attr,build-status"`
	Labels       string    `jsonapi:"attr,labels"`
	Manifest     string    `jsonapi:"attr,manifest"`
	InsertedAt   time.Time `jsonapi:"attr,inserted-at"`
	UpdatedAt    time.Time `jsonapi:"attr,updated-at"`
}
