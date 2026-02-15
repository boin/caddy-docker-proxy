#!/bin/bash

set -e

# Simple deploy helper for caddy-docker-proxy
# Usage:
#   ./deploy.sh build [tag]
#   ./deploy.sh push [tag]
#   ./deploy.sh build-push [tag]
#
# Defaults:
#   REGISTRY=registry.ttd
#   IMAGE_NAME=caddy-docker-proxy
#   tag=latest

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

COMMAND="$1"; shift || true
TAG="${1:-latest}"

REGISTRY="${REGISTRY:-registry.ttd}"
IMAGE_NAME="${IMAGE_NAME:-caddy-docker-proxy}"
FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${TAG}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

usage() {
  cat <<EOF
Usage: $0 <command> [tag]

Commands:
  build       Build Docker image using root Dockerfile
  push        Push previously built image
  build-push  Build and then push image

Environment:
  REGISTRY    Registry host (default: registry.ttd)
  IMAGE_NAME  Image name (default: caddy-docker-proxy)
EOF
}

if [[ -z "$COMMAND" ]]; then
  usage
  exit 1
fi

case "$COMMAND" in
  build)
    log_info "Building artifacts via build.sh ..."
    ARTIFACTS=${ARTIFACTS:-./artifacts} ./build.sh

    log_info "Building Docker image ${FULL_IMAGE} from root Dockerfile ..."
    docker build \
      -t "${FULL_IMAGE}" \
      -f Dockerfile \
      .
    ;;

  push)
    log_info "Pushing image ${FULL_IMAGE} ..."
    docker push "${FULL_IMAGE}"
    ;;

  build-push)
    "$0" build "$TAG"

    # Extra tags similar to previous build-push.sh
    if [[ "$TAG" == "latest" ]]; then
      if GIT_HASH=$(git rev-parse --short HEAD 2>/dev/null); then
        COMMIT_IMAGE="${REGISTRY}/${IMAGE_NAME}:${GIT_HASH}"
        log_info "Tagging with commit hash: ${COMMIT_IMAGE}"
        docker tag "${FULL_IMAGE}" "${COMMIT_IMAGE}"
      fi

      DATE_TAG=$(date +%Y%m%d)
      DATE_IMAGE="${REGISTRY}/${IMAGE_NAME}:${DATE_TAG}"
      log_info "Tagging with date: ${DATE_IMAGE}"
      docker tag "${FULL_IMAGE}" "${DATE_IMAGE}"
    fi

    log_info "Pushing image(s) ..."
    docker push "${FULL_IMAGE}"
    if [[ "$TAG" == "latest" ]]; then
      [[ -n "$COMMIT_IMAGE" ]] && docker push "${COMMIT_IMAGE}"
      docker push "${DATE_IMAGE}"
    fi
    ;;

  *)
    log_error "Unknown command: $COMMAND"
    usage
    exit 1
    ;;

esac

log_info "Done"
