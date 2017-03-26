FROM travix/gocd-agent:17.3.0-alpine

MAINTAINER estafette.io

# copy builder
COPY estafette-ci-builder /usr/bin/
