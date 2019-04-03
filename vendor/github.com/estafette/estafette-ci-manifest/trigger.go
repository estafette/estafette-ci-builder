package manifest

import (
	"fmt"
	"regexp"
	"strings"
)

// EstafetteTrigger represents a trigger of any supported type and what action to take if the trigger fired
type EstafetteTrigger struct {
	Pipeline *EstafettePipelineTrigger `yaml:"pipeline,omitempty" json:"pipeline,omitempty"`
	Release  *EstafetteReleaseTrigger  `yaml:"release,omitempty" json:"release,omitempty"`
	Git      *EstafetteGitTrigger      `yaml:"git,omitempty" json:"git,omitempty"`
	Docker   *EstafetteDockerTrigger   `yaml:"docker,omitempty" json:"docker,omitempty"`
	Cron     *EstafetteCronTrigger     `yaml:"cron,omitempty" json:"cron,omitempty"`

	BuildAction   *EstafetteTriggerBuildAction   `yaml:"builds,omitempty" json:"builds,omitempty"`
	ReleaseAction *EstafetteTriggerReleaseAction `yaml:"releases,omitempty" json:"releases,omitempty"`
}

// EstafettePipelineTrigger fires for pipeline changes and applies filtering to limit when this results in an action
type EstafettePipelineTrigger struct {
	Event  string `yaml:"event,omitempty" json:"event,omitempty"`
	Status string `yaml:"status,omitempty" json:"status,omitempty"`
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`
}

// EstafetteReleaseTrigger fires for pipeline releases and applies filtering to limit when this results in an action
type EstafetteReleaseTrigger struct {
	Event  string `yaml:"event,omitempty" json:"event,omitempty"`
	Status string `yaml:"status,omitempty" json:"status,omitempty"`
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
	Target string `yaml:"target,omitempty" json:"target,omitempty"`
}

// EstafetteGitTrigger fires for git repository changes and applies filtering to limit when this results in an action
type EstafetteGitTrigger struct {
	Event      string `yaml:"event,omitempty" json:"event,omitempty"`
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty"`
	Branch     string `yaml:"branch,omitempty" json:"branch,omitempty"`
}

// EstafetteDockerTrigger fires for docker image changes and applies filtering to limit when this results in an action
type EstafetteDockerTrigger struct {
	Event string `yaml:"event,omitempty" json:"event,omitempty"`
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
	Tag   string `yaml:"tag,omitempty" json:"tag,omitempty"`
}

// EstafetteCronTrigger fires at intervals specified by the cron expression
type EstafetteCronTrigger struct {
	Expression string `yaml:"expression,omitempty" json:"expression,omitempty"`
}

// EstafetteTriggerBuildAction determines what builds when the trigger fires
type EstafetteTriggerBuildAction struct {
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`
}

// EstafetteTriggerReleaseAction determines what releases when the trigger fires
type EstafetteTriggerReleaseAction struct {
	Target string `yaml:"target,omitempty" json:"target,omitempty"`
	Action string `yaml:"action,omitempty" json:"action,omitempty"`
}

// SetDefaults sets defaults for EstafetteTrigger
func (t *EstafetteTrigger) SetDefaults(triggerType, targetName string) {
	if t.Pipeline != nil {
		t.Pipeline.SetDefaults()
	}
	if t.Release != nil {
		t.Release.SetDefaults()
	}
	if t.Git != nil {
		t.Git.SetDefaults()
	}
	if t.Docker != nil {
		t.Docker.SetDefaults()
	}
	if t.Cron != nil {
		t.Cron.SetDefaults()
	}

	switch triggerType {
	case "build":
		if t.BuildAction == nil {
			t.BuildAction = &EstafetteTriggerBuildAction{}
		}
		t.BuildAction.SetDefaults()
		break
	case "release":
		if t.ReleaseAction == nil {
			t.ReleaseAction = &EstafetteTriggerReleaseAction{}
		}
		t.ReleaseAction.SetDefaults(targetName)
		break
	}
}

// SetDefaults sets defaults for EstafettePipelineTrigger
func (p *EstafettePipelineTrigger) SetDefaults() {
	if p.Event == "" {
		p.Event = "finished"
	}
	if p.Status == "" {
		p.Status = "succeeded"
	}
	if p.Branch == "" {
		p.Branch = "master"
	}
}

// SetDefaults sets defaults for EstafetteReleaseTrigger
func (r *EstafetteReleaseTrigger) SetDefaults() {
	if r.Event == "" {
		r.Event = "finished"
	}
	if r.Status == "" {
		r.Status = "succeeded"
	}
}

// SetDefaults sets defaults for EstafetteGitTrigger
func (g *EstafetteGitTrigger) SetDefaults() {
	if g.Event == "" {
		g.Event = "push"
	}
	if g.Branch == "" {
		g.Branch = "master"
	}
}

// SetDefaults sets defaults for EstafetteDockerTrigger
func (d *EstafetteDockerTrigger) SetDefaults() {
}

// SetDefaults sets defaults for EstafetteCronTrigger
func (c *EstafetteCronTrigger) SetDefaults() {
}

// SetDefaults sets defaults for EstafetteTriggerBuildAction
func (b *EstafetteTriggerBuildAction) SetDefaults() {
	if b.Branch == "" {
		b.Branch = "master"
	}
}

// SetDefaults sets defaults for EstafetteTriggerReleaseAction
func (r *EstafetteTriggerReleaseAction) SetDefaults(targetName string) {
	r.Target = targetName
}

// Validate checks if EstafetteTrigger is valid
func (t *EstafetteTrigger) Validate(triggerType, targetName string) (err error) {

	numberOfTypes := 0

	if t.Pipeline == nil &&
		t.Release == nil &&
		t.Git == nil &&
		t.Docker == nil &&
		t.Cron == nil {
		return fmt.Errorf("Set at least a 'pipeline', 'release', 'git', 'docker' or 'cron' trigger")
	}

	if t.Pipeline != nil {
		err = t.Pipeline.Validate()
		if err != nil {
			return err
		}
		numberOfTypes++
	}
	if t.Release != nil {
		err = t.Release.Validate()
		if err != nil {
			return err
		}
		numberOfTypes++
	}
	if t.Git != nil {
		err = t.Git.Validate()
		if err != nil {
			return err
		}
		numberOfTypes++
	}
	if t.Docker != nil {
		err = t.Docker.Validate()
		if err != nil {
			return err
		}
		numberOfTypes++
	}
	if t.Cron != nil {
		err = t.Cron.Validate()
		if err != nil {
			return err
		}
		numberOfTypes++
	}

	if numberOfTypes != 1 {
		return fmt.Errorf("Do not specify more than one type of trigger 'pipeline', 'release', 'git', 'docker' or 'cron' per trigger object")
	}

	switch triggerType {
	case "build":
		if t.BuildAction == nil {
			return fmt.Errorf("For a build trigger set the 'builds' property")
		}
		if t.ReleaseAction != nil {
			return fmt.Errorf("For a build trigger do not set the 'releases' property")
		}
		err = t.BuildAction.Validate()
		if err != nil {
			return err
		}

		break
	case "release":
		if t.ReleaseAction == nil {
			return fmt.Errorf("For a release trigger set the 'releases' property")
		}
		if t.BuildAction != nil {
			return fmt.Errorf("For a release trigger do not set the 'builds' property")
		}
		err = t.ReleaseAction.Validate(targetName)
		if err != nil {
			return err
		}
		break
	}

	return nil
}

// Validate checks if EstafettePipelineTrigger is valid
func (p *EstafettePipelineTrigger) Validate() (err error) {
	if p.Event != "started" && p.Event != "finished" {
		return fmt.Errorf("Set pipeline.event in your trigger to 'started' or 'finished'")
	}
	if p.Event == "finished" && p.Status != "succeeded" && p.Status != "failed" {
		return fmt.Errorf("Set pipeline.status in your trigger to 'succeeded' or 'failed' for event 'finished'")
	}
	if p.Name == "" {
		return fmt.Errorf("Set pipeline.name in your trigger to a full qualified pipeline name, i.e. github.com/estafette/estafette-ci-manifest")
	}
	return nil
}

// Validate checks if EstafetteReleaseTrigger is valid
func (r *EstafetteReleaseTrigger) Validate() (err error) {
	if r.Event != "started" && r.Event != "finished" {
		return fmt.Errorf("Set release.event in your trigger to 'started' or 'finished'")
	}
	if r.Event == "finished" && r.Status != "succeeded" && r.Status != "failed" {
		return fmt.Errorf("Set release.status in your trigger to 'succeeded' or 'failed' for event 'finished'")
	}
	if r.Name == "" {
		return fmt.Errorf("Set release.name in your trigger to a full qualified pipeline name, i.e. github.com/estafette/estafette-ci-manifest")
	}
	if r.Target == "" {
		return fmt.Errorf("Set release.target in your trigger to a release target name on the pipeline set by release.name")
	}
	return nil
}

// Validate checks if EstafetteGitTrigger is valid
func (g *EstafetteGitTrigger) Validate() (err error) {
	if g.Event != "push" {
		return fmt.Errorf("Set git.event in your trigger to 'push'")
	}
	if g.Repository == "" {
		return fmt.Errorf("Set git.repository in your trigger to a full qualified git repository name, i.e. github.com/estafette/estafette-ci-manifest")
	}
	return nil
}

// Validate checks if EstafetteDockerTrigger is valid
func (d *EstafetteDockerTrigger) Validate() (err error) {
	return nil
}

// Validate checks if EstafetteCronTrigger is valid
func (c *EstafetteCronTrigger) Validate() (err error) {
	return nil
}

// Validate checks if EstafetteTriggerBuildAction is valid
func (b *EstafetteTriggerBuildAction) Validate() (err error) {
	return nil
}

// Validate checks if EstafetteTriggerReleaseAction is valid
func (r *EstafetteTriggerReleaseAction) Validate(targetName string) (err error) {

	if r.Target != targetName {
		return fmt.Errorf("The target in your releases action should have defaulted to '%v'", targetName)
	}

	return nil
}

// Fires indicates whether EstafettePipelineTrigger fires for an EstafettePipelineEvent
func (p *EstafettePipelineTrigger) Fires(e *EstafettePipelineEvent) bool {

	// compare event as regex
	eventMatched, err := regexp.MatchString(p.Event, fmt.Sprintf("^%v$", e.Event))
	if err != nil || !eventMatched {
		return false
	}

	if p.Event == "finished" {
		// compare status as regex
		statusMatched, err := regexp.MatchString(p.Status, fmt.Sprintf("^%v$", e.Status))
		if err != nil || !statusMatched {
			return false
		}
	}

	// compare name case insensitive
	nameMatches := strings.EqualFold(p.Name, fmt.Sprintf("%v/%v/%v", e.RepoSource, e.RepoOwner, e.RepoName))
	if !nameMatches {
		return false
	}

	// compare branch as regex
	branchMatched, err := regexp.MatchString(p.Branch, fmt.Sprintf("^%v$", e.Branch))
	if err != nil || !branchMatched {
		return false
	}

	return true
}

// Fires indicates whether EstafetteReleaseTrigger fires for an EstafetteReleaseEvent
func (r *EstafetteReleaseTrigger) Fires(e *EstafetteReleaseEvent) bool {
	// compare event as regex
	eventMatched, err := regexp.MatchString(r.Event, fmt.Sprintf("^%v$", e.Event))
	if err != nil || !eventMatched {
		return false
	}

	if r.Event == "finished" {
		// compare status as regex
		statusMatched, err := regexp.MatchString(r.Status, fmt.Sprintf("^%v$", e.Status))
		if err != nil || !statusMatched {
			return false
		}
	}

	// compare name case insensitive
	nameMatches := strings.EqualFold(r.Name, fmt.Sprintf("%v/%v/%v", e.RepoSource, e.RepoOwner, e.RepoName))
	if !nameMatches {
		return false
	}

	// compare branch as regex
	branchMatched, err := regexp.MatchString(r.Target, fmt.Sprintf("^%v$", e.Target))
	if err != nil || !branchMatched {
		return false
	}

	return true
}

// Fires indicates whether EstafetteGitTrigger fires for an EstafetteGitEvent
func (g *EstafetteGitTrigger) Fires(e *EstafetteGitEvent) bool {
	return false
}

// Fires indicates whether EstafetteDockerTrigger fires for an EstafetteDockerEvent
func (d *EstafetteDockerTrigger) Fires(e *EstafetteDockerEvent) bool {
	return false
}

// Fires indicates whether EstafetteCronTrigger fires for an EstafetteCronEvent
func (c *EstafetteCronTrigger) Fires(e *EstafetteCronEvent) bool {
	return false
}
