#!/bin/bash
set -e
if [ -z "${BUILD_VERSION}" ];then
        echo 'Please set BUILD_VERSION.'
        exit 1
fi
mkdir -p output
#go build -tags product,musl,netgo,osusergo -ldflags="-w -s -X ${AGENT_PACKAGE}.Version=${BUILD_VERSION} -linkmode external -extldflags='-static'" -o build/cloud-guard-agent
GOARCH=amd64 go build -tags product,musl,netgo,osusergo -ldflags="-w -s -linkmode external -extldflags='-static'" -o output/collector-linux-amd64-${BUILD_VERSION}.plg
#GOARCH=arm64 go build -o output/collector-linux-arm64-${BUILD_VERSION}.plg
