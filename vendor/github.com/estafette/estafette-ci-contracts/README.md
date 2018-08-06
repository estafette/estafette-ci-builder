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
git config core.hooksPath hooks
brew install go
brew install protobuf
go get -u google.golang.org/grpc
go get -u github.com/golang/protobuf/protoc-gen-go
```

Before committing your changes run

```bash
go test `go list ./... | grep -v /vendor/`
```

The repo has a pre-commit hook that executes the following in order to generate the protobuf/grpc code for golang.

```bash
protoc --go_out=plugins=grpc:. *.proto
```