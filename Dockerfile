FROM docker:19.03.8-dind

LABEL maintainer="estafette.io" \
      description="The ${ESTAFETTE_GIT_NAME} is the component that runs builds as defined in the .estafette.yaml manifest"

RUN addgroup docker \
    && mkdir -p /estafette-entrypoints \
    && apk update \
    && apk add --no-cache --upgrade \
        openssl \
    && rm -rf /var/cache/apk/* \
    && docker version || true

# copy builder & startup script
COPY ${ESTAFETTE_GIT_NAME} /
COPY templates /entrypoint-templates

ENV ESTAFETTE_CI_SERVER="estafette" \
    ESTAFETTE_WORKDIR="/estafette-work" \
    ESTAFETTE_LOG_FORMAT="v3"

WORKDIR ${ESTAFETTE_WORKDIR}

ENTRYPOINT ["/${ESTAFETTE_GIT_NAME}"]