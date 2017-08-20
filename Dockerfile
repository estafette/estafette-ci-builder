FROM docker:17.06.0-ce-dind

MAINTAINER estafette.io

RUN addgroup docker

ENV ESTAFETTE_CI_SERVER="estafette"

# copy builder & startup script
COPY estafette-ci-builder /

WORKDIR /estafette-work

ENTRYPOINT ["/estafette-ci-builder"]