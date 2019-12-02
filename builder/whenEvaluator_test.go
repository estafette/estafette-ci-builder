package builder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	whenEvaluator = NewWhenEvaluator(envvarHelper)
)

func TestWhenEvaluator(t *testing.T) {

	t.Run("ReturnsFalseIfInputIsEmpty", func(t *testing.T) {

		envvarHelper.UnsetEstafetteEnvvars()

		// act
		result, err := whenEvaluator.Evaluate("name", "", make(map[string]interface{}, 0))

		assert.NotNil(t, err)
		assert.False(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithoutParameters", func(t *testing.T) {

		envvarHelper.UnsetEstafetteEnvvars()

		// act
		result, _ := whenEvaluator.Evaluate("name", "3 > 2", make(map[string]interface{}, 0))

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithParameters", func(t *testing.T) {

		envvarHelper.UnsetEstafetteEnvvars()
		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"
		parameters["trigger"] = ""
		parameters["status"] = "succeeded"

		// act
		result, _ := whenEvaluator.Evaluate("name", "status == 'succeeded' && branch == 'master'", parameters)

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithParameters", func(t *testing.T) {

		envvarHelper.UnsetEstafetteEnvvars()
		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"
		parameters["trigger"] = ""
		parameters["status"] = "succeeded"

		// act
		result, _ := whenEvaluator.Evaluate("name", "status == 'succeeded' && branch == 'master'", parameters)

		assert.True(t, result)
	})
}

func TestWhenParameters(t *testing.T) {

	t.Run("ReturnsMapWithBranchEqualToBranchWithoutTrailingNewline", func(t *testing.T) {

		envvarHelper.UnsetEstafetteEnvvars()
		envvarHelper.SetEstafetteGlobalEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")

		// act
		parameters := whenEvaluator.GetParameters()

		assert.False(t, strings.Contains(parameters["branch"].(string), "\n"))
	})

	t.Run("ReturnsMapWithAction", func(t *testing.T) {

		envvarHelper.UnsetEstafetteEnvvars()
		envvarHelper.SetEstafetteGlobalEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_RELEASE_ACTION", "deploy-canary")

		// act
		parameters := whenEvaluator.GetParameters()

		assert.Equal(t, "deploy-canary", parameters["action"])
	})

	t.Run("ReturnsMapWithStatusSetToSucceededByDefault", func(t *testing.T) {

		envvarHelper.UnsetEstafetteEnvvars()
		envvarHelper.SetEstafetteGlobalEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")

		// act
		parameters := whenEvaluator.GetParameters()

		assert.Equal(t, "succeeded", parameters["status"])
	})
}
