FROM travix/gocd-agent:17.3.0-alpine

MAINTAINER estafette.io

# copy builder
COPY estafette-ci-builder /opt/estafette/

RUN sed -i 's#export PATH=#export PATH=/opt/estafette:#' /etc/profile