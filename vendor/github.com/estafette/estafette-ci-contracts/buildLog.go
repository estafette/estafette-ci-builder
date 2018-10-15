package contracts

import "time"

// BuildLog represents a build log for a specific revision
type BuildLog struct {
	ID           string         `json:"id,omitempty"`
	RepoSource   string         `json:"repoSource"`
	RepoOwner    string         `json:"repoOwner"`
	RepoName     string         `json:"repoName"`
	RepoBranch   string         `json:"repoBranch"`
	RepoRevision string         `json:"repoRevision"`
	BuildID      string         `json:"buildID"`
	Steps        []BuildLogStep `json:"steps"`
	InsertedAt   time.Time      `json:"insertedAt"`
}

// BuildLogStep represents the logs for a single step of a pipeline
type BuildLogStep struct {
	Step         string                   `json:"step"`
	Image        *BuildLogStepDockerImage `json:"image"`
	RunIndex     int                      `json:"runIndex,omitempty"`
	Duration     time.Duration            `json:"duration"`
	LogLines     []BuildLogLine           `json:"logLines"`
	ExitCode     int64                    `json:"exitCode"`
	Status       string                   `json:"status"`
	AutoInjected bool                     `json:"autoInjected,omitempty"`
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

// TailLogLine returns a log line for streaming logs to gui during a build
type TailLogLine struct {
	Step    string                   `json:"step"`
	LogLine *BuildLogLine            `json:"logLine,omitempty"`
	Image   *BuildLogStepDockerImage `json:"image,omitempty"`
}
