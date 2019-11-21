package main

import (
	"errors"

	"github.com/Knetic/govaluate"
	"github.com/rs/zerolog/log"
)

// WhenEvaluator evaluates when clauses from the manifest
type WhenEvaluator interface {
	Evaluate(string, string, map[string]interface{}) (bool, error)
	GetParameters() map[string]interface{}
}

type whenEvaluatorImpl struct {
	envvarHelper EnvvarHelper
}

// NewWhenEvaluator returns a new WhenEvaluator
func NewWhenEvaluator(envvarHelper EnvvarHelper) WhenEvaluator {
	return &whenEvaluatorImpl{
		envvarHelper: envvarHelper,
	}
}

func (we *whenEvaluatorImpl) Evaluate(pipelineName, input string, parameters map[string]interface{}) (result bool, err error) {

	if input == "" {
		return false, errors.New("When expression is empty")
	}

	log.Info().Msgf("[%v] Evaluating when expression \"%v\" with parameters \"%v\"", pipelineName, input, parameters)

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

func (we *whenEvaluatorImpl) GetParameters() map[string]interface{} {

	parameters := make(map[string]interface{}, 3)
	parameters["branch"] = we.envvarHelper.getEstafetteEnv("ESTAFETTE_GIT_BRANCH")
	parameters["trigger"] = we.envvarHelper.getEstafetteEnv("ESTAFETTE_TRIGGER")
	parameters["status"] = we.envvarHelper.getEstafetteEnv("ESTAFETTE_BUILD_STATUS")
	parameters["action"] = we.envvarHelper.getEstafetteEnv("ESTAFETTE_RELEASE_ACTION")
	parameters["server"] = we.envvarHelper.getCiServer()

	return parameters
}
