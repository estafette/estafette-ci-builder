package main

var (
	dockerRunner = NewDockerRunner(envvarHelper, NewObfuscator(secretHelper), true)
)

func init() {
	dockerRunner.createDockerClient()
}
