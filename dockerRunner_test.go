package main

import (
	"testing"

	"github.com/docker/go-connections/nat"
	contracts "github.com/estafette/estafette-ci-contracts"
	"github.com/stretchr/testify/assert"
)

var (
	dockerRunner = NewDockerRunner(envvarHelper, NewObfuscator(secretHelper), true, contracts.BuilderConfig{DockerNetwork: &contracts.DockerNetworkConfig{Name: "estafette-integration", Subnet: "192.168.4.1/24", Gateway: "192.168.4.1"}}, make(chan struct{}))
)

func init() {
	dockerRunner.createDockerClient()
}

func TestParsePortSpecs(t *testing.T) {

	t.Run("ParsePortSpecs", func(t *testing.T) {

		// act
		exposedPorts, bindings, err := nat.ParsePortSpecs([]string{"127.0.0.1:8000:8080/tcp"})

		if assert.Nil(t, err, "Error %v", err) {
			assert.Equal(t, 1, len(exposedPorts))
			assert.Equal(t, struct{}{}, exposedPorts["8080/tcp"])
			assert.Equal(t, 1, len(bindings))
			assert.Equal(t, 1, len(bindings["8080/tcp"]))
			assert.Equal(t, "8000", bindings["8080/tcp"][0].HostPort)
			assert.Equal(t, "127.0.0.1", bindings["8080/tcp"][0].HostIP)
		}
	})
}
