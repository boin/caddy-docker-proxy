#!/bin/bash

set -e

. ../functions.sh

# e2e test for Host Status feature
# This script is intended to be run via `tests/run.sh`

trap "docker rm -f caddy-e2e whoami1-e2e whoami2-e2e api-e2e >/dev/null 2>&1 || true" EXIT

{
  # Build local test image
  docker build -t caddy-docker-proxy:local -f Dockerfile.test ../.. &&

  # Start caddy with host status enabled
  docker run -d --name caddy-e2e \
    -p 8080:8080 \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e CADDY_DOCKER_HOST_STATUS_URL=/caddy/hosts \
    -e CADDY_DOCKER_HOST_STATUS_TEMPLATE=/etc/caddy/hostview.html \
    -e CADDY_DOCKER_HOST_STATUS_ADDR=:8080 \
    caddy-docker-proxy:local &&

  # Start test services
  docker run -d --name whoami1-e2e \
    -l caddy=whoami1.localhost \
    -l "caddy.reverse_proxy={{upstreams 80}}" \
    traefik/whoami &&

  docker run -d --name whoami2-e2e \
    -l caddy=whoami2.localhost \
    -l "caddy.reverse_proxy={{upstreams 80}}" \
    traefik/whoami &&

  docker run -d --name api-e2e \
    -l caddy=api.localhost \
    -l "caddy.reverse_proxy={{upstreams 80}}" \
    traefik/whoami &&

  # Wait for caddy to pick up containers and host status server to be ready
  retry bash -lc 'curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/caddy/hosts | grep -q 200' &&

  # Verify HTML page contains expected title
  retry bash -lc 'curl -s http://localhost:8080/caddy/hosts | grep -q "Caddy 主机列表"' &&

  # Verify JSON data contains all expected hosts
  retry bash -lc 'curl -s http://localhost:8080/caddy/hosts/data | grep -q "\"whoami1.localhost\""' &&
  retry bash -lc 'curl -s http://localhost:8080/caddy/hosts/data | grep -q "\"whoami2.localhost\""' &&
  retry bash -lc 'curl -s http://localhost:8080/caddy/hosts/data | grep -q "\"api.localhost\""'

} || {
  echo "Test failed"
  exit 1
}
