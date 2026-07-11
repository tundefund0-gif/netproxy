#!/bin/sh
set -e
mkdir -p dist

echo "Building linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "dist/netproxy_amd64" .

echo "Building linux/arm64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "dist/netproxy_arm64" .

echo "Building linux/arm (ARMv7)..."
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o "dist/netproxy_armv7" .

echo "Size:"
ls -lh dist/
