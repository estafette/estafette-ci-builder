FROM docker:19.03.5-dind

LABEL maintainer="estafette.io" \
      description="The ${ESTAFETTE_GIT_NAME} is the component that runs builds as defined in the .estafette.yaml manifest"

RUN addgroup docker

# copy builder & startup script
COPY ${ESTAFETTE_GIT_NAME} /

ENV ESTAFETTE_CI_SERVER="estafette" \
    ESTAFETTE_WORKDIR="/estafette-work"

WORKDIR ${ESTAFETTE_WORKDIR}

ENTRYPOINT ["/${ESTAFETTE_GIT_NAME}"]