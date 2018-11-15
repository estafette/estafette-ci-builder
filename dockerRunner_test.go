package main

import contracts "github.com/estafette/estafette-ci-contracts"

var (
	dockerRunner = NewDockerRunner(envvarHelper, NewObfuscator(secretHelper), true, contracts.BuilderConfig{}, make(chan struct{}))
)

func init() {
	dockerRunner.createDockerClient()
}
