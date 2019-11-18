package main

import (
	"context"
	"os"
	"testing"

	contracts "github.com/estafette/estafette-ci-contracts"
	mft "github.com/estafette/estafette-ci-manifest"
	"github.com/stretchr/testify/assert"
)

var (
	pipelineRunner = NewPipelineRunner(envvarHelper, whenEvaluator, dockerRunner, true, make(chan struct{}), tailLogsChannel)
)

func TestRunStages(t *testing.T) {

	t.Run("ReturnsErrorWhenManifestHasNoStages", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		_, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.NotNil(t, err, "Error: %v", err)
	})

	t.Run("ReturnsResultWithInnerResultForEachStageInManifest", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(result.StageResults))
	})

	t.Run("ReturnsResultWithoutErrorsWhenStagesSucceeded", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithSucceededPipelineResultWhenStagesSucceeded", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "SUCCEEDED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenStagesFailed", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithFailedPipelineResultWhenStagesFailed", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "FAILED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultWithoutErrorsWhenStagesSkipped", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithSkippedStageResultWhenStagesSkipped", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "SKIPPED", result.StageResults[0].Status)
	})

	t.Run("ReturnsResultForAllStagesWhenFirstStageFails", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.Equal(t, "FAILED", result.StageResults[0].Status)
		assert.Equal(t, "SKIPPED", result.StageResults[1].Status)
		assert.Equal(t, "SUCCEEDED", result.StageResults[2].Status)
	})

	t.Run("ReturnsResultWithErrorsWhenFirstStageFailsAndSecondSucceeds", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 1"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'succeeded'"})
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestStep3", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Commands: []string{"exit 0"}, When: "status == 'failed'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithoutErrorsWhenStagesSucceededAfterRetrial", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		cmd := "if [ -f retried ]; then rm retried && exit 0; else touch retried && exit 1; fi;"
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestRetryStep", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Retries: 1, Commands: []string{cmd}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.False(t, result.HasAggregatedErrors())
	})

	t.Run("ReturnsResultWithErrorsWhenStagesFailedAfterRetrial", func(t *testing.T) {

		envvarHelper.unsetEstafetteEnvvars()
		envvarHelper.setEstafetteEnv("ESTAFETTE_BUILD_STATUS", "succeeded")
		cmd := "exit 1"
		manifest := &mft.EstafetteManifest{}
		manifest.Stages = append(manifest.Stages, &mft.EstafetteStage{Name: "TestRetryStep2", ContainerImage: "busybox:latest", Shell: "/bin/sh", WorkingDirectory: "/estafette-work", Retries: 1, Commands: []string{cmd}, When: "status == 'succeeded'"})
		envvars := map[string]string{}
		dir, _ := os.Getwd()

		// act
		result, err := pipelineRunner.runStages(context.Background(), 0, manifest.Stages, dir, envvars)

		assert.Nil(t, err)
		assert.True(t, result.HasAggregatedErrors())
	})
}

func TestGetMainBuildLogStep(t *testing.T) {

	t.Run("ReturnsNilIfBuildLogsStepsIsEmpty", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: make([]*contracts.BuildLogStep, 0),
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
		}

		// act
		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfStageDoesNotExistInBuildLogsSteps", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-b",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
		}

		// act
		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsBuildLogsStepForStageIfExistsInBuildLogsSteps", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
		}

		// act
		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

		assert.NotNil(t, buildLogStep)
		assert.Equal(t, "stage-a", buildLogStep.Step)
	})

	t.Run("ReturnsNilIfStageDoesNotExistInBuildLogsStepsWithRunIndex", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:     "stage-a",
					RunIndex: 0,
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:     "stage-a",
			RunIndex: 1,
		}

		// act
		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsBuildLogsStepForStageIfExistsInBuildLogsStepsWithRunIndex", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:     "stage-a",
					RunIndex: 1,
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:     "stage-a",
			RunIndex: 1,
		}

		// act
		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

		assert.NotNil(t, buildLogStep)
		assert.Equal(t, "stage-a", buildLogStep.Step)
	})

	t.Run("ReturnsNilIfParentStageDoesNotExistInBuildLogsSteps", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-b",
		}

		// act
		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsBuildLogsStepForStageIfParentStageExistsInBuildLogsSteps", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
		}

		// act
		buildLogStep := pipelineRunner.getMainBuildLogStep(tailLogLine)

		assert.NotNil(t, buildLogStep)
		assert.Equal(t, "stage-a", buildLogStep.Step)
	})
}

func TestGetNestedBuildLogStep(t *testing.T) {

	t.Run("ReturnsNilIfBuildLogsStepsIsEmpty", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: make([]*contracts.BuildLogStep, 0),
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "stage",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfDepthIsZero", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       0,
			Type:        "stage",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfParentStageExistsButNestedStageDoesNot", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-stage-1",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "stage",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfParentStageExistsButNestedStageDoesNotAndServiceWithSameNameExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-stage-1",
						},
					},
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-stage-0",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "stage",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNestedStepIfParentStageAndNestedStageExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-stage-0",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "stage",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogStep(tailLogLine)

		assert.NotNil(t, buildLogStep)
		assert.Equal(t, "nested-stage-0", buildLogStep.Step)
	})
}

func TestGetNestedBuildLogService(t *testing.T) {

	t.Run("ReturnsNilIfBuildLogsStepsIsEmpty", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: make([]*contracts.BuildLogStep, 0),
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfDepthIsZero", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       0,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfParentStageExistsButNestedStageDoesNot", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-1",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNilIfParentStageExistsButNestedStageDoesNotAndServiceWithSameNameExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-0",
						},
					},
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-1",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.Nil(t, buildLogStep)
	})

	t.Run("ReturnsNestedStepIfParentStageAndNestedStageExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-0",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Depth:       1,
			Type:        "service",
		}

		// act
		buildLogStep := pipelineRunner.getNestedBuildLogService(tailLogLine)

		assert.NotNil(t, buildLogStep)
		assert.Equal(t, "nested-service-0", buildLogStep.Step)
	})
}

func TestUpsertTailLogLine(t *testing.T) {

	t.Run("AddsMainStageIfDoesNotExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: make([]*contracts.BuildLogStep, 0),
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("DoesNotReaddMainStageIfAlreadyExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("AddsMainStageIfDoesNotExistWithRunIndex", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:     "stage-a",
					RunIndex: 0,
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:     "stage-a",
			RunIndex: 1,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 2, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 0, pipelineRunner.buildLogSteps[0].RunIndex)
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[1].Step)
		assert.Equal(t, 1, pipelineRunner.buildLogSteps[1].RunIndex)
	})

	t.Run("AddsMainStageIfDoesNotExistForNestedStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("AddsMainStageIfDoesNotExistForNestedService", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "service",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
	})

	t.Run("AddsNestedStageIfDoesNotExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps[0].NestedSteps))
		assert.Equal(t, "nested-stage-0", pipelineRunner.buildLogSteps[0].NestedSteps[0].Step)
	})

	t.Run("DoesNotReaddNestedStageIfAlreadyExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-stage-0",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps[0].NestedSteps))
		assert.Equal(t, "nested-stage-0", pipelineRunner.buildLogSteps[0].NestedSteps[0].Step)
	})

	t.Run("AddsNestedServiceIfDoesNotExist", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        "service",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps[0].Services))
		assert.Equal(t, "nested-service-0", pipelineRunner.buildLogSteps[0].Services[0].Step)
	})

	t.Run("DoesNotReaddNestedServiceIfAlreadyExists", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-0",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        "service",
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps))
		assert.Equal(t, "stage-a", pipelineRunner.buildLogSteps[0].Step)
		assert.Equal(t, 1, len(pipelineRunner.buildLogSteps[0].Services))
		assert.Equal(t, "nested-service-0", pipelineRunner.buildLogSteps[0].Services[0].Step)
	})

	t.Run("AddLogLineToMainStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					LogLines: []contracts.BuildLogLine{
						contracts.BuildLogLine{
							LineNumber: 1,
							Text:       "Hi this is the first line",
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step: "stage-a",
			LogLine: &contracts.BuildLogLine{
				LineNumber: 2,
				Text:       "Hey I'd like to add a second line",
			},
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 2, len(pipelineRunner.buildLogSteps[0].LogLines))
		assert.Equal(t, 1, pipelineRunner.buildLogSteps[0].LogLines[0].LineNumber)
		assert.Equal(t, 2, pipelineRunner.buildLogSteps[0].LogLines[1].LineNumber)
	})

	t.Run("AddLogLineToNestedStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-stage-0",
							LogLines: []contracts.BuildLogLine{
								contracts.BuildLogLine{
									LineNumber: 1,
									Text:       "Hi this is the first line",
								},
							},
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
			LogLine: &contracts.BuildLogLine{
				LineNumber: 2,
				Text:       "Hey I'd like to add a second line",
			},
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 2, len(pipelineRunner.buildLogSteps[0].NestedSteps[0].LogLines))
		assert.Equal(t, 1, pipelineRunner.buildLogSteps[0].NestedSteps[0].LogLines[0].LineNumber)
		assert.Equal(t, 2, pipelineRunner.buildLogSteps[0].NestedSteps[0].LogLines[1].LineNumber)
	})

	t.Run("AddLogLineToNestedService", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step: "nested-service-0",
							LogLines: []contracts.BuildLogLine{
								contracts.BuildLogLine{
									LineNumber: 1,
									Text:       "Hi this is the first line",
								},
							},
						},
					},
				},
			},
		}
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        "service",
			LogLine: &contracts.BuildLogLine{
				LineNumber: 2,
				Text:       "Hey I'd like to add a second line",
			},
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, 2, len(pipelineRunner.buildLogSteps[0].Services[0].LogLines))
		assert.Equal(t, 1, pipelineRunner.buildLogSteps[0].Services[0].LogLines[0].LineNumber)
		assert.Equal(t, 2, pipelineRunner.buildLogSteps[0].Services[0].LogLines[1].LineNumber)
	})

	t.Run("SetStatusForMainStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step:   "stage-a",
					Status: "PENDING",
				},
			},
		}
		status := "RUNNING"
		tailLogLine := contracts.TailLogLine{
			Step:   "stage-a",
			Status: &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, "RUNNING", pipelineRunner.buildLogSteps[0].Status)
	})

	t.Run("SetStatusForNestedStage", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					NestedSteps: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-stage-0",
							Status: "PENDING",
						},
					},
				},
			},
		}
		status := "RUNNING"
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-stage-0",
			ParentStage: "stage-a",
			Type:        "stage",
			Status:      &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, "RUNNING", pipelineRunner.buildLogSteps[0].NestedSteps[0].Status)
	})

	t.Run("SetStatusForNestedService", func(t *testing.T) {

		pipelineRunner := pipelineRunnerImpl{
			buildLogSteps: []*contracts.BuildLogStep{
				&contracts.BuildLogStep{
					Step: "stage-a",
					Services: []*contracts.BuildLogStep{
						&contracts.BuildLogStep{
							Step:   "nested-service-0",
							Status: "PENDING",
						},
					},
				},
			},
		}
		status := "RUNNING"
		tailLogLine := contracts.TailLogLine{
			Step:        "nested-service-0",
			ParentStage: "stage-a",
			Type:        "service",
			Status:      &status,
		}

		// act
		pipelineRunner.upsertTailLogLine(tailLogLine)

		assert.Equal(t, "RUNNING", pipelineRunner.buildLogSteps[0].Services[0].Status)
	})
}
