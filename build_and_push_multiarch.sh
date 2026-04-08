#!/usr/bin/env bash
set -euo pipefail

# Usage: ./build_and_push_multiarch.sh <dockerhub-username> <repo> <tag>
if [ "$#" -lt 3 ]; then
  echo "Usage: $0 <dockerhub-username> <repo> <tag>"
  exit 1
fi
USER="$1"
REPO="$2"
TAG="$3"
IMAGE_NAME="$USER/$REPO:$TAG"

# ensure buildx available
docker buildx version >/dev/null

# create builder if not exists
docker buildx inspect multi-builder >/dev/null 2>&1 || docker buildx create --name multi-builder --use

# build and push multi-arch image for linux/amd64 and linux/arm64
docker buildx build --platform linux/amd64,linux/arm64 -t "$IMAGE_NAME" --push .

echo "multi-arch image pushed: $IMAGE_NAME"
