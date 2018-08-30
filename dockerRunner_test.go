package main

var (
	dockerRunner = NewDockerRunner(envvarHelper)
)

func init() {
	dockerRunner.createDockerClient()
}
