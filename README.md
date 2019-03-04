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
go test
go mod tidy
go mod vendor
```