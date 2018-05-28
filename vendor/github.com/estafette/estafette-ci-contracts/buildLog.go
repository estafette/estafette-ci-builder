package contracts

import "time"

// BuildLog represents a build log for a specific revision
type BuildLog struct {
	ID           string         `json:"id,omitempty" jsonapi:"primary,build-logs"`
	RepoSource   string         `json:"repoSource" jsonapi:"attr,repo-source"`
	RepoOwner    string         `json:"repoOwner" jsonapi:"attr,repo-owner"`
	RepoName     string         `json:"repoName" jsonapi:"attr,repo-name"`
	RepoBranch   string         `json:"repoBranch" jsonapi:"attr,repo-branch"`
	RepoRevision string         `json:"repoRevision" jsonapi:"attr,repo-revision"`
	Steps        []BuildLogStep `json:"steps" jsonapi:"attr,steps"`
	InsertedAt   time.Time      `json:"insertedAt" jsonapi:"attr,inserted-at"`
}

// BuildLogStep represents the logs for a single step of a pipeline
type BuildLogStep struct {
	Step     string                   `json:"step"`
	Image    *BuildLogStepDockerImage `json:"image"`
	Duration time.Duration            `json:"duration"`
	LogLines []BuildLogLine           `json:"logLines"`
	ExitCode int64                    `json:"exitCode"`
	Status   string                   `json:"status"`
}

// BuildLogStepDockerImage represents info about the docker image used for a step
type BuildLogStepDockerImage struct {
	Name         string        `json:"name"`
	Tag          string        `json:"tag"`
	IsPulled     bool          `json:"isPulled"`
	ImageSize    int64         `json:"imageSize"`
	PullDuration time.Duration `json:"pullDuration"`
	Error        string        `json:"error,omitempty"`
}

// BuildLogLine has low level log information
type BuildLogLine struct {
	Timestamp  time.Time `json:"timestamp"`
	StreamType string    `json:"streamType"`
	Text       string    `json:"text"`
}
