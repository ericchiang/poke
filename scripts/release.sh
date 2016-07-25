#!/bin/bash -e

mkdir -p dist

PROJ="poke"
ORG_PATH="github.com/ericchiang"
REPO_PATH="${ORG_PATH}/${PROJ}"

sudo rkt run \
    --set-env=CGO_ENABLED=0 \
    --set-env=GOBIN=/go/src/${REPO_PATH}/dist \
    --volume src,kind=host,source=${PWD} \
    --mount volume=src,target=/go/src/${REPO_PATH} \
    --insecure-options=image \
    docker://golang:1.6.3 \
    --exec go -- install -v -a \
	${REPO_PATH}/cmd/${PROJ}/...
