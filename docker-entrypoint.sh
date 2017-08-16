#!/bin/sh
set -e

# SIGTERM-handler
sigterm_handler() {
    # stop estafette-ci-builder
    estafette_pid=$(pgrep estafette-ci-builder)
    echo "Received SIGTERM, killing estafette-ci-builder with pid $estafette_pid..."
    kill -SIGTERM "$estafette_pid"
    wait "$estafette_pid"
    echo "Killed estafette-ci-builder"
}

# setup handlers
echo "Setting up signal handlers..."
trap 'kill ${!}; sigterm_handler' 15 # SIGTERM

# run dockerd
echo "Starting docker daemon..."
dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2375 --storage-driver=$STORAGE_DRIVER 2>&1 &

# wait for docker.sock to be ready
echo "Waiting for docker daemon..."
while [ ! -e /var/run/docker.sock ] ; do
  echo "."
  sleep 1s
done

# run estafette-ci-builder
/estafette-ci-builder &

# wait forever
while true
do
  tail -f /dev/null & wait ${!}
done