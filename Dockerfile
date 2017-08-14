FROM docker:17.06.0-ce-dind

MAINTAINER estafette.io

RUN apk add --no-cache \
    git

ENV ESTAFETTE_CI_SERVER="estafette"

# copy builder
COPY estafette-ci-builder /

ENTRYPOINT ["/estafette-ci-builder"]