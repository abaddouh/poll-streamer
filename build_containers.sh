#!/bin/bash

set -e

NGINX_TAG=docker-registry.ops.pe/streamer:nginx-latest
STREAMER_TAG=docker-registry.ops.pe/streamer:streamer-latest

echo "Building nginx..."
docker buildx build --platform linux/arm64 --file="./docker/nginx/Dockerfile" --tag=$NGINX_TAG ./docker/nginx

echo "Building streamer..."
docker buildx build --platform linux/arm64 --file="./docker/streamer/Dockerfile" --tag=$STREAMER_TAG .

echo "All Containers built."
