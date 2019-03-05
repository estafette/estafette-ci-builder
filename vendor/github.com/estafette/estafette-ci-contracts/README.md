# Estafette CI

The `estafette-ci-contracts` library is part of the Estafette CI system documented at https://estafette.io.

Please file any issues related to Estafette CI at https://github.com/estafette/estafette-ci-central/issues

## Estafette-ci-contracts

This library has contracts for requests / responses between various components of the Estafette CI system.

## Development

To start development run

```bash
git clone git@github.com:estafette/estafette-ci-contracts.git
cd estafette-ci-contracts
```

Before committing your changes run

```bash
go test
go mod tidy
go mod vendor
```