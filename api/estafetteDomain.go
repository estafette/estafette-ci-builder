package api

import contracts "github.com/estafette/estafette-ci-contracts"

// EstafetteCiBuilderEvent represents a finished estafette build
type EstafetteCiBuilderEvent struct {
	JobName      string           `json:"job_name"`
	PodName      string           `json:"pod_name,omitempty"`
	RepoSource   string           `json:"repo_source,omitempty"`
	RepoOwner    string           `json:"repo_owner,omitempty"`
	RepoName     string           `json:"repo_name,omitempty"`
	RepoBranch   string           `json:"repo_branch,omitempty"`
	RepoRevision string           `json:"repo_revision,omitempty"`
	ReleaseID    string           `json:"release_id,omitempty"`
	BuildID      string           `json:"build_id,omitempty"`
	BuildStatus  contracts.Status `json:"build_status,omitempty"`
}
