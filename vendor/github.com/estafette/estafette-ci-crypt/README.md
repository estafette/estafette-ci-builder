# Estafette CI

The `estafette-ci-crypt` library is part of the Estafette CI system documented at https://estafette.io.

Please file any issues related to Estafette CI at https://github.com/estafette/estafette-ci-central/issues

## Estafette-ci-crypt

This library provides encrypt / decrypt functionality for Estafette CI secrets; it uses AES-256 encryption.

## Development

To start development run

```bash
git clone git@github.com:estafette/estafette-ci-crypt.git
cd estafette-ci-crypt
```

Before committing your changes run

```bash
go test
go mod tidy
```