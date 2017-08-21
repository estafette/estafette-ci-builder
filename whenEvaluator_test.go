package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhenEvaluator(t *testing.T) {

	t.Run("ReturnsFalseIfInputIsEmpty", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TEST_")
		whenEvaluator := NewWhenEvaluator(envvarHelper)

		// act
		result, err := whenEvaluator.evaluate("", make(map[string]interface{}, 0))

		assert.NotNil(t, err)
		assert.False(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithoutParameters", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TEST_")
		whenEvaluator := NewWhenEvaluator(envvarHelper)

		// act
		result, _ := whenEvaluator.evaluate("3 > 2", make(map[string]interface{}, 0))

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithParameters", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TEST_")
		whenEvaluator := NewWhenEvaluator(envvarHelper)

		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"
		parameters["trigger"] = ""
		parameters["status"] = "succeeded"

		// act
		result, _ := whenEvaluator.evaluate("status == 'succeeded' && branch == 'master'", parameters)

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithParameters", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TEST_")
		whenEvaluator := NewWhenEvaluator(envvarHelper)

		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"
		parameters["trigger"] = ""
		parameters["status"] = "succeeded"

		// act
		result, _ := whenEvaluator.evaluate("status == 'succeeded' && branch == 'master'", parameters)

		assert.True(t, result)
	})
}

func TestWhenParameters(t *testing.T) {

	t.Run("ReturnsMapWithBranchEqualToBranchWithoutTrailingNewline", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TEST_")
		whenEvaluator := NewWhenEvaluator(envvarHelper)

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteGlobalEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")

		// act
		parameters := whenEvaluator.getParameters()

		assert.False(t, strings.Contains(parameters["branch"].(string), "\n"))

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})

	t.Run("ReturnsMapWithStatusSetToSucceededByDefault", func(t *testing.T) {

		envvarHelper := NewEnvvarHelper("TEST_")
		whenEvaluator := NewWhenEvaluator(envvarHelper)

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteGlobalEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")

		// act
		parameters := whenEvaluator.getParameters()

		assert.Equal(t, "succeeded", parameters["status"])

		// clean up
		envvarHelper.unsetEstafetteEnvvars()
	})
}
