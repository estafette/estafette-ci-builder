# Estafette CI builder

The **CI builder** is the component that interprets the `.estafette.yaml` file and executes the pipelines specified in there.

At this time the yaml file consist of two top-level sections, _labels_ and _pipelines_.

## Labels

_Labels_ are not required, but are useful to keep pipelines slightly less applications specific by using the labels as variables. In the future labels will be the mechanism to easily filter application pipelines by label values.

Any of the labels can be used in all the pipeline steps with the environment variable `ESTAFETTE_LABEL_<LABEL NAME>` in snake-casing. 

## Pipelines

The _pipelines_ section allows you to define multiple steps to run within public Docker containers. Within each step you specify the public Docker image to use and the commands to execute within the Docker container. Your cloned repository is mounted within the docker container at `/estafette-work` by default, but you can override it with the `workDir` setting in case you need your code to live in a certain directory structure like with certain golang projects.

All files in the working directory are passed from step to step. Any files stored outside the working directory are not passed on. This means that - for example - when fetching NuGet packages with .NET core you specify the packages directory to be stored within the working directory, so it's available in the following steps.

For each step you can use a different Docker image, but it's best practice to minimize the number of different images / versions in order to save time on downloading all the Docker images.

Pipeline steps run in sequence. With the `when` property you can specify a logical expression - in golang format - to determine whether the step should run or be skipped. This is useful to, for example, only push docker containers to a registry for certain branches. Or to fire a notification if any of the previous steps has failed. The default `when` condition is `status == 'succeeded'`.

# Examples

## golang

```yaml
labels:
  app: <APP NAME>
  language: golang

pipelines:
  build:
    image: golang:1.8.1-alpine
    workDir: /go/src/github.com/estafette/${ESTAFETTE_LABEL_APP}
    commands:
    - go test `go list ./... | grep -v /vendor/`
    - CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.version=${ESTAFETTE_BUILD_VERSION} -X main.revision=${ESTAFETTE_GIT_REVISION} -X main.branch=${ESTAFETTE_GIT_BRANCH} -X main.buildDate=${ESTAFETTE_BUILD_DATETIME}" -o ./publish/${ESTAFETTE_LABEL_APP} .
```

## csharp .net core

```yaml
labels:
  app: <APP NAME>
  language: dotnet-core

pipelines:
  restore:
    image: microsoft/dotnet:1.1.1-sdk
    commands:
    - dotnet restore --source https://www.nuget.org/api/v1 --source http://nuget-server.tooling/nuget --packages .nuget/packages

  build:
    image: microsoft/dotnet:1.1.1-sdk
    commands:
    - dotnet build --configuration Release --version-suffix ${ESTAFETTE_BUILD_VERSION_PATCH}

  unit-tests:
    image: microsoft/dotnet:1.1.1-sdk
    commands:
    - dotnet test --configuration Release --no-build test/<unit test project directory/<unit test project file>.csproj

  integration-tests:
    image: microsoft/dotnet:1.1.1-sdk
    commands:
    - dotnet test --configuration Release --no-build test/<integration test project directory>/<integration test project file>.csproj

  publish:
    image: microsoft/dotnet:1.1.1-sdk
    commands:
    - dotnet publish src/<publisheable project directory> --configuration Release --runtime debian.8-x64 --version-suffix ${ESTAFETTE_BUILD_VERSION_PATCH} --output ./publish
```

## python

```yaml
labels:
  app: <APP NAME>
  language: python

pipelines:
  build:
    image: python:3.4.6-alpine
    commands:
    - python -m compileall
```

## java / maven

```yaml
labels:
  app: <APP NAME>
  language: java

pipelines:
  build:
    image: maven:3.3.9-jdk-8-alpine
    commands:
    - mvn -B clean verify -Dmaven.repo.remote=https://<sonatype nexus server>/content/groups/public
```

## node js

```yaml
labels:
  app: <APP NAME>
  language: nodejs

pipelines:
  build:
    image: node:7.8.0-alpine
    commands:
    - npm install --verbose
```

## dockerize & push for master branch

```yaml
labels:
  app: <APP NAME>

pipelines:
  bake:
    image: docker:17.04.0-ce
    commands:
    - cp /etc/ssl/certs/ca-certificates.crt .
    - docker build -t estafette/${ESTAFETTE_LABEL_APP}:${ESTAFETTE_BUILD_VERSION} .

  push-to-docker-hub:
    image: docker:17.04.0-ce
    commands:
    - docker login --username=${ESTAFETTE_DOCKER_HUB_USERNAME} --password="${ESTAFETTE_DOCKER_HUB_PASSWORD}"
    - docker push estafette/${ESTAFETTE_LABEL_APP}:${ESTAFETTE_BUILD_VERSION}
    when:
      status == 'succeeded' &&
      branch == 'master'
```

## notification on failure

```yaml
labels:
  app: <APP NAME>

pipelines:
  slack-notify:
    image: docker:17.04.0-ce
    commands:
    - 'curl -X POST --data-urlencode ''payload={"channel": "#build-status", "username": "<DOCKER HUB USERNAME>", "text": "Build ''${ESTAFETTE_BUILD_VERSION}'' for ''${ESTAFETTE_LABEL_APP}'' has failed!"}'' ${ESTAFETTE_SLACK_WEBHOOK}'
    when:
      status == 'failed'
```
