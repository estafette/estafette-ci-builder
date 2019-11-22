FROM docker:19.03.5-dind

LABEL maintainer="estafette.io" \
      description="The ${ESTAFETTE_GIT_NAME} is the component that runs builds as defined in the .estafette.yaml manifest"

RUN addgroup docker \
    && mkdir -p /estafette-entrypoints

# copy builder & startup script
COPY ${ESTAFETTE_GIT_NAME} /
COPY templates /estafette-templates

ENV ESTAFETTE_CI_SERVER="estafette" \
    ESTAFETTE_WORKDIR="/estafette-work" \
    ESTAFETTE_LOG_FORMAT="console"

WORKDIR ${ESTAFETTE_WORKDIR}

ENTRYPOINT ["/${ESTAFETTE_GIT_NAME}"]