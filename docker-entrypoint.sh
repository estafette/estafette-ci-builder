#!/bin/sh
set -e

# SIGTERM-handler
sigterm_handler() {
  # kubernetes sends a sigterm, where nginx needs SIGQUIT for graceful shutdown
  gopid=$(cat /var/lib/go-agent/go-agent.pid)
  echo "Gracefully shutting down go.cd agent with pid ${gopid}..."
  kill -15 $gopid
  echo "Finished shutting down go.cd agent!"
}

# setup handlers
echo "Setting up signal handlers..."
trap 'kill ${!}; sigterm_handler' 15 # SIGTERM

# autoregister agent with server
if [ -n "$AGENT_KEY" ]
then
  mkdir -p /var/lib/go-agent/config
  echo "agent.auto.register.key=$AGENT_KEY" > /var/lib/go-agent/config/autoregister.properties
  if [ -n "$AGENT_RESOURCES" ]
  then
    echo "agent.auto.register.resources=$AGENT_RESOURCES" >> /var/lib/go-agent/config/autoregister.properties
  fi
  if [ -n "$AGENT_ENVIRONMENTS" ]
  then
    echo "agent.auto.register.environments=$AGENT_ENVIRONMENTS" >> /var/lib/go-agent/config/autoregister.properties
  fi
  if [ -n "$AGENT_HOSTNAME" ]
  then
    echo "agent.auto.register.hostname=$AGENT_HOSTNAME" >> /var/lib/go-agent/config/autoregister.properties
  fi
fi

# log to std out instead of file
cat >/var/lib/go-agent/log4j.properties <<EOL
log4j.rootCategory=INFO, ConsoleAppender
log4j.logger.net.sourceforge.cruisecontrol=INFO
log4j.logger.com.thoughtworks.go=INFO
log4j.logger.org.springframework.context.support=INFO
log4j.logger.httpclient.wire=INFO
# console output...
log4j.appender.ConsoleAppender=org.apache.log4j.RollingFileAppender
log4j.appender.ConsoleAppender.layout=org.apache.log4j.PatternLayout
log4j.appender.ConsoleAppender.layout.ConversionPattern=%d{ISO8601} [%-9t] %-5p %-16c{4}:%L %x- %m%n
EOL

# wait for server to be available
until curl -ksLo /dev/null "${GO_SERVER_URL}"
do
  sleep 5
  echo "Waiting for ${GO_SERVER_URL}"
done

# run dockerd
echo "Starting docker daemon..."
dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2375 --storage-driver=overlay &

# run go.cd agent
echo "Starting go.cd agent..."
/bin/bash /var/lib/go-agent/agent.sh &

# store pid
gopid=$!
echo "Started go.cd agent with pid ${gopid}..."
echo $gopid > /var/lib/go-agent/go-agent.pid

# wait forever
while true
do
  tail -f /dev/null & wait ${!}
done