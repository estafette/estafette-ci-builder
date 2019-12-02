package builder

// CIBuilder runs builds for different types of integrations
type CIBuilder interface {
	RunReadinessProbe(protocol, host string, port int, path, hostname string, timeoutSeconds int) error
	RunEstafetteBuildJob() error
	RunGocdAgentBuild() error
	RunEstafetteCLIBuild() error
}

type ciBuilderImpl struct {
}

// NewCIBuilder returns a new CIBuilder
func NewCIBuilder() CIBuilder {
	return &ciBuilderImpl{}
}

func (b *ciBuilderImpl) RunReadinessProbe(protocol, host string, port int, path, hostname string, timeoutSeconds int) error {
	return WaitForReadiness(protocol, host, port, path, hostname, timeoutSeconds)
}

func (b *ciBuilderImpl) RunEstafetteBuildJob() error {
	return nil
}

func (b *ciBuilderImpl) RunGocdAgentBuild() error {
	return nil
}

func (b *ciBuilderImpl) RunEstafetteCLIBuild() error {
	return nil
}
