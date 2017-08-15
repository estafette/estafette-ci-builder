package main

import (
	"fmt"
	"os/exec"

	"github.com/rs/zerolog/log"
)

func gitCloneRevision(gitURL, gitBranch, gitRevision string) (err error) {

	log.Info().
		Str("url", gitURL).
		Str("branch", gitBranch).
		Str("revision", gitRevision).
		Msgf("Cloning git repository %v to branch %v and revision %v...", gitURL, gitBranch, gitRevision)

	// git clone
	args := []string{"clone", "--depth=50", fmt.Sprintf("--branch=%v", gitBranch), gitURL, "/estafette-work"}
	err = exec.Command("git", args...).Run()
	if err != nil {
		return
	}

	// checkout specific revision
	if gitRevision != "" {

		args := []string{"checkout", "--quiet", "--force", gitRevision}
		checkoutCommand := exec.Command("git", args...)
		checkoutCommand.Dir = "/estafette-work"
		err = checkoutCommand.Run()
		if err != nil {
			return
		}
	}

	log.Info().
		Str("url", gitURL).
		Str("branch", branch).
		Str("revision", revision).
		Msgf("Finished cloning git repository %v to branch %v and revision %v", gitURL, gitBranch, gitRevision)

	return
}
