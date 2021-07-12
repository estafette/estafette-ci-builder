package builder

import (
	"errors"
	"os"

	"github.com/Knetic/govaluate"
	"github.com/rs/zerolog/log"
)

// WhenEvaluator evaluates when clauses from the manifest
type WhenEvaluator interface {
	Evaluate(string, string, map[string]interface{}) (bool, error)
	GetParameters() map[string]interface{}
}

type whenEvaluator struct {
	envvarHelper EnvvarHelper
}

// NewWhenEvaluator returns a new WhenEvaluator
func NewWhenEvaluator(envvarHelper EnvvarHelper) WhenEvaluator {
	return &whenEvaluator{
		envvarHelper: envvarHelper,
	}
}

func (we *whenEvaluator) Evaluate(pipelineName, input string, parameters map[string]interface{}) (result bool, err error) {

	if input == "" {
		return false, errors.New("When expression is empty")
	}

	log.Debug().Msgf("[%v] Evaluating when expression \"%v\" with parameters \"%v\"", pipelineName, input, parameters)

	// replace estafette envvars in when clause
	input = os.Expand(input, we.envvarHelper.getEstafetteEnv)

	expression, err := govaluate.NewEvaluableExpression(input)
	if err != nil {
		return
	}

	r, err := expression.Evaluate(parameters)

	log.Debug().Msgf("[%v] Result of when expression \"%v\" is \"%v\"", pipelineName, input, r)

	if result, ok := r.(bool); ok {
		return result, err
	}

	return false, errors.New("Result of evaluating when expression is not of type boolean")
}

func (we *whenEvaluator) GetParameters() map[string]interface{} {

	parameters := make(map[string]interface{}, 3)
	parameters["branch"] = we.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_BRANCH")
	parameters["trigger"] = we.envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER")
	parameters["status"] = we.envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_STATUS")
	parameters["action"] = we.envvarHelper.getEstafetteEnv("ESTAFETTE_RELEASE_ACTION")
	parameters["server"] = we.envvarHelper.GetCiServer()

	return parameters
}
