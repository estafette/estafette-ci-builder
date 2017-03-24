FROM travix/gocd-agent-gcloud:17.3.0

MAINTAINER estafette.io

COPY estafette-ci-builder /opt/estafette/

# add to path
RUN sed -i -e "s_export PATH=_export PATH=/opt/estafette:_" /var/go/.profile