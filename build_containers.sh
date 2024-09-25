#!/bin/bash

set -e

NGINX_TAG=docker-registry.ops.pe/poll-streamer:nginx-latest
STREAMER_TAG=docker-registry.ops.pe/poll-streamer:streamer-latest

echo "Building nginx..."
docker buildx build --platform linux/arm64 --file="./docker/nginx/Dockerfile" --push --tag=$NGINX_TAG ./docker/nginx

echo "Building streamer..."
docker buildx build --platform linux/arm64 --file="./docker/streamer/Dockerfile" --push --tag=$STREAMER_TAG .

echo "All Containers built."
