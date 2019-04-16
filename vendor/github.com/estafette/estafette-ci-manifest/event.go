package manifest

import "time"

// EstafettePipelineEvent fires for pipeline changes
type EstafettePipelineEvent struct {
	BuildVersion string `yaml:"buildVersion,omitempty" json:"buildVersion,omitempty"`
	RepoSource   string `yaml:"repoSource,omitempty" json:"repoSource,omitempty"`
	RepoOwner    string `yaml:"repoOwner,omitempty" json:"repoOwner,omitempty"`
	RepoName     string `yaml:"repoName,omitempty" json:"repoName,omitempty"`
	Branch       string `yaml:"repoBranch,omitempty" json:"repoBranch,omitempty"`
	Status       string `yaml:"status,omitempty" json:"status,omitempty"`
	Event        string `yaml:"event,omitempty" json:"event,omitempty"`
}

// EstafetteReleaseEvent fires for pipeline releases
type EstafetteReleaseEvent struct {
	ReleaseVersion string `yaml:"releaseVersion,omitempty" json:"releaseVersion,omitempty"`
	RepoSource     string `yaml:"repoSource,omitempty" json:"repoSource,omitempty"`
	RepoOwner      string `yaml:"repoOwner,omitempty" json:"repoOwner,omitempty"`
	RepoName       string `yaml:"repoName,omitempty" json:"repoName,omitempty"`
	Target         string `yaml:"target,omitempty" json:"target,omitempty"`
	Status         string `yaml:"status,omitempty" json:"status,omitempty"`
	Event          string `yaml:"event,omitempty" json:"event,omitempty"`
}

// EstafetteGitEvent fires for git repository changes
type EstafetteGitEvent struct {
	Event      string `yaml:"event,omitempty" json:"event,omitempty"`
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty"`
	Branch     string `yaml:"branch,omitempty" json:"branch,omitempty"`
}

// EstafetteDockerEvent fires for docker image changes
type EstafetteDockerEvent struct {
	Event string `yaml:"event,omitempty" json:"event,omitempty"`
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
	Tag   string `yaml:"tag,omitempty" json:"tag,omitempty"`
}

// EstafetteCronEvent fires at intervals specified by the cron expression
type EstafetteCronEvent struct {
	Time time.Time `yaml:"time,omitempty" json:"time,omitempty"`
}

// EstafetteManualEvent fires when a user manually triggers a build or release
type EstafetteManualEvent struct {
	UserID string `yaml:"userID,omitempty" json:"userID,omitempty"`
}

// EstafetteEvent is a container for any trigger event
type EstafetteEvent struct {
	Pipeline *EstafettePipelineEvent `yaml:"pipeline,omitempty" json:"pipeline,omitempty"`
	Release  *EstafetteReleaseEvent  `yaml:"release,omitempty" json:"release,omitempty"`
	Git      *EstafetteGitEvent      `yaml:"git,omitempty" json:"git,omitempty"`
	Docker   *EstafetteDockerEvent   `yaml:"docker,omitempty" json:"docker,omitempty"`
	Cron     *EstafetteCronEvent     `yaml:"cron,omitempty" json:"cron,omitempty"`
	Manual   *EstafetteManualEvent   `yaml:"manual,omitempty" json:"manual,omitempty"`
}
