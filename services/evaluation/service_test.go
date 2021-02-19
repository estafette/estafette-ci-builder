package evaluation

import (
	"context"
	"strings"
	"testing"

	"github.com/estafette/estafette-ci-builder/clients/envvar"
	"github.com/estafette/estafette-ci-builder/clients/obfuscation"
	crypt "github.com/estafette/estafette-ci-crypt"
	"github.com/stretchr/testify/assert"
)

func TestWhenEvaluator(t *testing.T) {

	t.Run("ReturnsFalseIfInputIsEmpty", func(t *testing.T) {

		evaluationService, _ := getEvaluationService()

		// act
		result, err := evaluationService.Evaluate("name", "", make(map[string]interface{}, 0))

		assert.NotNil(t, err)
		assert.False(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithoutParameters", func(t *testing.T) {

		evaluationService, _ := getEvaluationService()

		// act
		result, _ := evaluationService.Evaluate("name", "3 > 2", make(map[string]interface{}, 0))

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithParameters", func(t *testing.T) {

		evaluationService, _ := getEvaluationService()
		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"
		parameters["trigger"] = ""
		parameters["status"] = "succeeded"

		// act
		result, _ := evaluationService.Evaluate("name", "status == 'succeeded' && branch == 'master'", parameters)

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithParameters", func(t *testing.T) {

		evaluationService, _ := getEvaluationService()
		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"
		parameters["trigger"] = ""
		parameters["status"] = "succeeded"

		// act
		result, _ := evaluationService.Evaluate("name", "status == 'succeeded' && branch == 'master'", parameters)

		assert.True(t, result)
	})

	t.Run("ReturnsFalseIfInputIsMalformed", func(t *testing.T) {

		evaluationService, _ := getEvaluationService()
		parameters := make(map[string]interface{}, 3)
		parameters["action"] = "deploy-canary"
		parameters["branch"] = "some-branch"
		parameters["server"] = "estafette"
		parameters["status"] = "succeeded"
		parameters["trigger"] = ""

		// act
		result, err := evaluationService.Evaluate("name", "action == 'deploy-canary' || action == 'deploy-stable' || action == 'rollback-canary' || action == 'restart-stable' || action == 'diff-stable || action == 'diff-simple", parameters)

		assert.NotNil(t, err)
		assert.False(t, result)
	})
}

func TestWhenParameters(t *testing.T) {

	t.Run("ReturnsMapWithBranchEqualToBranchWithoutTrailingNewline", func(t *testing.T) {

		evaluationService, envvarClient := getEvaluationService()
		envvarClient.SetEstafetteGlobalEnvvars()
		envvarClient.SetEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")

		// act
		parameters := evaluationService.GetParameters()

		assert.False(t, strings.Contains(parameters["branch"].(string), "\n"))
	})

	t.Run("ReturnsMapWithAction", func(t *testing.T) {

		evaluationService, envvarClient := getEvaluationService()
		envvarClient.SetEstafetteGlobalEnvvars()
		envvarClient.SetEstafetteEnv("ESTAFETTE_RELEASE_ACTION", "deploy-canary")

		// act
		parameters := evaluationService.GetParameters()

		assert.Equal(t, "deploy-canary", parameters["action"])
	})

	t.Run("ReturnsMapWithStatusSetToSucceededByDefault", func(t *testing.T) {

		evaluationService, envvarClient := getEvaluationService()
		envvarClient.SetEstafetteGlobalEnvvars()
		envvarClient.SetEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")

		// act
		parameters := evaluationService.GetParameters()

		assert.Equal(t, "succeeded", parameters["status"])
	})
}

func getEvaluationService() (Service, envvar.Client) {
	ctx := context.Background()
	secretHelper := crypt.NewSecretHelper("SazbwMf3NZxVVbBqQHebPcXCqrVn3DDp", false)
	obfuscationClient, _ := obfuscation.NewClient(ctx, secretHelper)
	envvarClient, _ := envvar.NewClient(ctx, "TESTPREFIX_", secretHelper, obfuscationClient)
	envvarClient.UnsetEstafetteEnvvars()

	evaluationService, _ := NewService(ctx, envvarClient)

	return evaluationService, envvarClient
}
