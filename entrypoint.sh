#!/usr/bin/env sh
set -e

DOCKER_SOCKET=${DOCKER_SOCKET:-tcp://dockerproxy:2375}
PORT=${PORT:-8080}
LOG_LEVEL=${LOG_LEVEL:-debug}

echo "Starting dockershocker with loglevel ${LOG_LEVEL} on port ${PORT} and socket ${DOCKER_SOCKET}"

./dockershocker -logLevel="$LOG_LEVEL" -port="$PORT" -dockerSocket="$DOCKER_SOCKET"
