FROM travix/gocd-agent:17.4.0-alpine

MAINTAINER estafette.io

# copy builder
COPY estafette-ci-builder /usr/bin/
