FROM docker:17.09.1-ce-dind

LABEL maintainer="estafette.io" \
      description="The estafette-ci-builder is the component that runs builds as defined in the .estafette.yaml manifest"

RUN addgroup docker

ENV ESTAFETTE_CI_SERVER="estafette"

# copy builder & startup script
COPY estafette-ci-builder /

WORKDIR /estafette-work

ENTRYPOINT ["/estafette-ci-builder"]