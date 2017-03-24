FROM docker:17.03.0-ce-dind

MAINTAINER estafette.io

# build time environment variables
ENV GO_VERSION=17.3.0 \
    GO_BUILD_VERSION=17.3.0-4704

# install go.cd agent
RUN apk --update-cache upgrade \
    && apk add --update-cache \
      openjdk8-jre-base \
      git \
      bash \
      curl \
    && rm /var/cache/apk/* \
    && curl -fSL "https://download.gocd.io/binaries/${GO_BUILD_VERSION}/generic/go-agent-${GO_BUILD_VERSION}.zip" -o /tmp/go-agent.zip \
    && unzip /tmp/go-agent.zip -d / \
    && rm /tmp/go-agent.zip \
    && mv go-agent-${GO_VERSION} /var/lib/go-agent \
    && mkdir -p /var/log/go-agent /var/go

# copy builder
COPY estafette-ci-builder /usr/bin/
COPY docker-entrypoint.sh /
RUN chmod 500 /docker-entrypoint.sh

# runtime environment variables
ENV AGENT_BOOTSTRAPPER_ARGS="-sslVerificationMode NONE" \
    AGENT_ENVIRONMENTS="" \
    AGENT_HOSTNAME="" \
    AGENT_KEY="" \
    AGENT_MAX_MEM=256m \
    AGENT_MEM=128m \
    AGENT_RESOURCES="" \
    GO_SERVER_URL=https://localhost:8154/go \
    HOME="/var/go"

CMD ["/docker-entrypoint.sh"]