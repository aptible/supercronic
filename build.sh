#!/usr/bin/env bash

set -x

mkdir -p dist
export GOOS="linux"
export CGO_ENABLED=0
VERSION=${1:-$(git describe --tags --always --dirty)}
for arch in amd64 386 arm arm64 loong64; do GOARCH="$arch" go build -ldflags="-X 'main.Version=$VERSION'" && file supercronic | grep 'statically linked' && mv supercronic "dist/supercronic-${GOOS}-${arch}"; done
pushd dist
ls -lah *
file *
sha1sum *
sha256sum *
popd
