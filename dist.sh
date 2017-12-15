#!/bin/bash

set -eo pipefail

mkdir -p bin

version=`go run awsc.go version | cut -d " " -f 2`

for arch in ${ALL_GOARCH}; do
  for platform in ${ALL_GOOS}; do
    file="bin/${NAME}-${version}.${platform}.${arch}"
    echo "Building ${file}"
    CGO_ENABLED=0 GOOS=${platform} GOARCH=${arch} ${GOBUILD} -o ${file}
  done
done

cd bin && shasum -a 256 * > ${NAME}.sha256sums
