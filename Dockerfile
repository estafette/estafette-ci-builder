FROM docker:17.06.0-ce-dind

MAINTAINER estafette.io

RUN apk add --no-cache \
    git

ENV ESTAFETTE_CI_SERVER="estafette" \
    STORAGE_DRIVER="overlay2"

# copy builder & startup script
COPY estafette-ci-builder /
COPY docker-entrypoint.sh /
RUN chmod 500 /docker-entrypoint.sh

WORKDIR /estafette-work

ENTRYPOINT ["/docker-entrypoint.sh"]