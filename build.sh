#!/bin/sh
set -e
NAME=netproxy
mkdir -p dist

echo "Building linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "dist/${NAME}_amd64" .

echo "Building linux/arm64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "dist/${NAME}_arm64" .

echo "Building linux/arm (ARMv7)..."
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o "dist/${NAME}_armv7" .

echo "Done:"
ls -lh dist/
