package evaluation

import (
	"context"
	"errors"
	"os"

	"github.com/Knetic/govaluate"
	"github.com/estafette/estafette-ci-builder/clients/envvar"
	"github.com/rs/zerolog/log"
)

// Service evaluates when clauses from the manifest
//go:generate mockgen -package=evaluation -destination ./mock.go -source=service.go
type Service interface {
	Evaluate(string, string, map[string]interface{}) (bool, error)
	GetParameters() map[string]interface{}
}

// NewService returns a new evaluation.Service
func NewService(ctx context.Context, envvarClient envvar.Client) (Service, error) {
	return &service{
		envvarClient: envvarClient,
	}, nil
}

type service struct {
	envvarClient envvar.Client
}

func (s *service) Evaluate(pipelineName, input string, parameters map[string]interface{}) (result bool, err error) {

	if input == "" {
		return false, errors.New("When expression is empty")
	}

	log.Info().Msgf("[%v] Evaluating when expression \"%v\" with parameters \"%v\"", pipelineName, input, parameters)

	// replace estafette envvars in when clause
	input = os.Expand(input, s.envvarClient.GetEstafetteEnv)

	expression, err := govaluate.NewEvaluableExpression(input)
	if err != nil {
		return
	}

	r, err := expression.Evaluate(parameters)

	log.Info().Msgf("[%v] Result of when expression \"%v\" is \"%v\"", pipelineName, input, r)

	if result, ok := r.(bool); ok {
		return result, err
	}

	return false, errors.New("Result of evaluating when expression is not of type boolean")
}

func (s *service) GetParameters() map[string]interface{} {

	parameters := make(map[string]interface{}, 3)
	parameters["branch"] = s.envvarClient.GetEstafetteEnv("ESTAFETTE_GIT_BRANCH")
	parameters["trigger"] = s.envvarClient.GetEstafetteEnv("ESTAFETTE_TRIGGER")
	parameters["status"] = s.envvarClient.GetEstafetteEnv("ESTAFETTE_BUILD_STATUS")
	parameters["action"] = s.envvarClient.GetEstafetteEnv("ESTAFETTE_RELEASE_ACTION")
	parameters["server"] = s.envvarClient.GetCiServer()

	return parameters
}
