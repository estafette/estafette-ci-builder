package main

import (
	"errors"
	"os"

	"github.com/Knetic/govaluate"
	"github.com/rs/zerolog/log"
)

func whenEvaluator(input string, parameters map[string]interface{}) (result bool, err error) {

	if input == "" {
		return false, errors.New("When expression is empty")
	}

	log.Info().Msgf("Evaluating when expression \"%v\" with parameters \"%v\"", input, parameters)

	expression, err := govaluate.NewEvaluableExpression(input)

	r, err := expression.Evaluate(parameters)

	log.Info().Msgf("Result of when expression \"%v\" is \"%v\"", input, r)

	if result, ok := r.(bool); ok {
		return result, err
	}

	return false, errors.New("Result of evaluating when expression is not of type boolean")
}

func whenParameters() map[string]interface{} {

	parameters := make(map[string]interface{}, 3)
	parameters["branch"] = os.Getenv("ESTAFETTE_GIT_BRANCH")
	parameters["trigger"] = os.Getenv("ESTAFETTE_TRIGGER")
	parameters["status"] = os.Getenv("ESTAFETTE_BUILD_STATUS")

	return parameters
}
