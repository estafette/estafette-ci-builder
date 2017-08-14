package main

import "os/exec"
import "fmt"

func gitCloneRevision(gitURL, gitBranch, gitRevision string) (err error) {

	fmt.Print("Cloning git repository...")

	// git clone
	args := []string{"clone", "--depth=50", fmt.Sprintf("--branch=%v", gitBranch), gitURL, "/estafette-work"}
	err = exec.Command("git", args...).Run()
	if err != nil {
		return
	}

	// checkout specific revision
	if gitRevision != "" {

		fmt.Printf("Checking out git commit %v...\n", gitRevision)

		args := []string{"checkout", "--quiet", "--force", gitRevision}
		checkoutCommand := exec.Command("git", args...)
		checkoutCommand.Dir = "/estafette-work"
		err = checkoutCommand.Run()
		if err != nil {
			return
		}
	}

	return
}
