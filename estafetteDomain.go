package main

import "time"

// EstafetteCiBuilderEvent represents a finished estafette build
type EstafetteCiBuilderEvent struct {
	JobName      string `json:"job_name"`
	RepoSource   string `json:"repo_source,omitempty"`
	RepoOwner    string `json:"repo_owner,omitempty"`
	RepoName     string `json:"repo_name,omitempty"`
	RepoBranch   string `json:"repo_branch,omitempty"`
	RepoRevision string `json:"repo_revision,omitempty"`
	ReleaseID    string `json:"release_id,omitempty"`
	BuildID      string `json:"build_id,omitempty"`
	BuildStatus  string `json:"build_status,omitempty"`
}

// BuildJobLogs represents the logs for a build job
type BuildJobLogs struct {
	RepoFullName string
	RepoBranch   string
	RepoRevision string
	RepoSource   string
	LogText      string
}

type buildJobLogLine struct {
	timestamp time.Time
	logLevel  string
	logText   string
}
