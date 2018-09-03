package main

var (
	dockerRunner = NewDockerRunner(envvarHelper, NewObfuscator(secretHelper))
)

func init() {
	dockerRunner.createDockerClient()
}
