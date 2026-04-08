#!/usr/bin/env bash
set -euo pipefail

# Usage: ./push_to_dockerhub.sh <dockerhub-username> <repo> <tag>
# Example: ./push_to_dockerhub.sh outsider json_compare latest
if [ "$#" -lt 3 ]; then
  echo "Usage: $0 <dockerhub-username> <repo> <tag>"
  exit 1
fi
USER="$1"
REPO="$2"
TAG="$3"
IMAGE_NAME="$USER/$REPO:$TAG"

echo "Building image $IMAGE_NAME..."
docker build -t "$IMAGE_NAME" .

echo "Pushing $IMAGE_NAME to Docker Hub..."
docker push "$IMAGE_NAME"

echo "Done."
