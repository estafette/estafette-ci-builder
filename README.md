# Estafette CI

The `estafette-ci-builder` component is part of the Estafette CI system documented at https://estafette.io.

Please file any issues related to Estafette CI at https://github.com/estafette/estafette-ci-central/issues

## Estafette-ci-builder

This component performs the actual builds as defined by the Estafette CI manifest in an application repository. It runs as a Kubernetes job.

## Development

To start development run

```bash
git clone git@github.com:estafette/estafette-ci-builder.git
cd estafette-ci-builder
```

Before committing your changes run

```bash
go test ./...
go mod tidy
```

## Docker golang library

With docker's golang engine not making use of golang modules it's pretty hard to get it to use the right version.

It's currently using the https://github.com/docker/docker-ce/releases/tag/v19.03.2 release by adding the following `replace` in the `go.mod` file:

```
replace github.com/docker/docker v1.13.1 => github.com/docker/engine v0.0.0-20190822205725-ed20165a37b4
```

To upgrade it to a new version find the new release on the https://github.com/docker/docker-ce/releases page, take the first 12 characters of the commit hash and update it in the replace statement:

```
replace github.com/docker/docker v1.13.1 => github.com/docker/engine <first 12 characters of commit hash>
```