package builder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhenEvaluator(t *testing.T) {

	t.Run("ReturnsFalseIfInputIsEmpty", func(t *testing.T) {

		_, _, _, whenEvaluator := getMocks()

		// act
		result, err := whenEvaluator.Evaluate("name", "", make(map[string]interface{}))

		assert.NotNil(t, err)
		assert.False(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithoutParameters", func(t *testing.T) {

		_, _, _, whenEvaluator := getMocks()

		// act
		result, _ := whenEvaluator.Evaluate("name", "3 > 2", make(map[string]interface{}))

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithParameters", func(t *testing.T) {

		_, _, _, whenEvaluator := getMocks()
		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"
		parameters["trigger"] = ""
		parameters["status"] = "succeeded"

		// act
		result, _ := whenEvaluator.Evaluate("name", "status == 'succeeded' && branch == 'master'", parameters)

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithParameters", func(t *testing.T) {

		_, _, _, whenEvaluator := getMocks()
		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"
		parameters["trigger"] = ""
		parameters["status"] = "succeeded"

		// act
		result, _ := whenEvaluator.Evaluate("name", "status == 'succeeded' && branch == 'master'", parameters)

		assert.True(t, result)
	})

	t.Run("ReturnsFalseIfInputIsMalformed", func(t *testing.T) {

		_, _, _, whenEvaluator := getMocks()
		parameters := make(map[string]interface{}, 3)
		parameters["action"] = "deploy-canary"
		parameters["branch"] = "some-branch"
		parameters["server"] = "estafette"
		parameters["status"] = "succeeded"
		parameters["trigger"] = ""

		// act
		result, err := whenEvaluator.Evaluate("name", "action == 'deploy-canary' || action == 'deploy-stable' || action == 'rollback-canary' || action == 'restart-stable' || action == 'diff-stable || action == 'diff-simple", parameters)

		assert.NotNil(t, err)
		assert.False(t, result)
	})
}

func TestWhenParameters(t *testing.T) {

	t.Run("ReturnsMapWithBranchEqualToBranchWithoutTrailingNewline", func(t *testing.T) {

		_, _, envvarHelper, whenEvaluator := getMocks()
		err := envvarHelper.SetEstafetteGlobalEnvvars()
		assert.Nil(t, err)
		err = envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		assert.Nil(t, err)

		// act
		parameters := whenEvaluator.GetParameters()

		assert.False(t, strings.Contains(parameters["branch"].(string), "\n"))
	})

	t.Run("ReturnsMapWithAction", func(t *testing.T) {

		_, _, envvarHelper, whenEvaluator := getMocks()
		err := envvarHelper.SetEstafetteGlobalEnvvars()
		assert.Nil(t, err)
		err = envvarHelper.setEstafetteEnv("ESTAFETTE_RELEASE_ACTION", "deploy-canary")
		assert.Nil(t, err)

		// act
		parameters := whenEvaluator.GetParameters()

		assert.Equal(t, "deploy-canary", parameters["action"])
	})

	t.Run("ReturnsMapWithStatusSetToSucceededByDefault", func(t *testing.T) {

		_, _, envvarHelper, whenEvaluator := getMocks()
		err := envvarHelper.SetEstafetteGlobalEnvvars()
		assert.Nil(t, err)
		err = envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		assert.Nil(t, err)

		// act
		parameters := whenEvaluator.GetParameters()

		assert.Equal(t, "succeeded", parameters["status"])
	})
}
