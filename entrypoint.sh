#!/usr/bin/env sh

set -e

echo "Starting dockershocker with loglevel ${LOG_LEVEL:-debug} on port ${PORT:-8080} and socket ${DOCKER_SOCKET:-tcp://dockerproxy:2375}"

./dockershocker -loglevel="${LOG_LEVEL:-debug}" -port="${PORT:-8080}" -socket="${DOCKER_SOCKET:-tcp://dockerproxy:2375}"
