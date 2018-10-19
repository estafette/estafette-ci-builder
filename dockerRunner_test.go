package main

import contracts "github.com/estafette/estafette-ci-contracts"

var (
	dockerRunner = NewDockerRunner(envvarHelper, NewObfuscator(secretHelper), true, contracts.BuilderConfig{})
)

func init() {
	dockerRunner.createDockerClient()
}
