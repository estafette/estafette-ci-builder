package manifest

// EstafettePipelineEvent fires for pipeline changes
type EstafettePipelineEvent struct {
	RepoSource string
	RepoOwner  string
	RepoName   string
	Branch     string
	Status     string
	Event      string
}

// EstafetteReleaseEvent fires for pipeline releases
type EstafetteReleaseEvent struct {
	RepoSource string
	RepoOwner  string
	RepoName   string
	Target     string
	Status     string
	Event      string
}

// EstafetteGitEvent fires for git repository changes
type EstafetteGitEvent struct {
	Event      string
	Repository string
	Branch     string
}

// EstafetteDockerEvent fires for docker image changes
type EstafetteDockerEvent struct {
	Event string
	Image string
	Tag   string
}

// EstafetteCronEvent fires at intervals specified by the cron expression
type EstafetteCronEvent struct {
	Expression string
}
