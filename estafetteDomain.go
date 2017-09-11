package main

// EstafetteCiBuilderEvent represents a finished estafette build
type EstafetteCiBuilderEvent struct {
	JobName string `json:"job_name"`
}

// BuildJobLogs represents the logs for a build job
type BuildJobLogs struct {
	RepoFullName string
	RepoBranch   string
	RepoRevision string
	RepoSource   string
	LogText      string
}
