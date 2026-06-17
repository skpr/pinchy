#!/usr/bin/env bash

PB_DIR="./proto/pb"
IMAGE="localhost/pinchy-protoc:latest"

echo "Generating protobuf code..."
rm -rf $PB_DIR
mkdir -p $PB_DIR
# Sadly can't get protoc on mac through mise.
docker build -t $IMAGE proto
docker run -w /data -v ./proto:/data $IMAGE /bin/bash -c 'protoc --go_out=./pb --go_opt=paths=source_relative --go-grpc_out=./pb --go-grpc_opt=paths=source_relative  *.proto'

echo "Generating manifests..."
controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

echo "Generating code..."
controller-gen object:headerFile=./hack/boilerplate.go.txt paths="./..."
client-gen --input-base=github.com/skpr/pinchy/apis \
           --input="pinchy/v1beta1" \
           --go-header-file=./hack/boilerplate.go.txt \
           --output-dir=internal \
           --output-pkg=github.com/skpr/pinchy/internal \
           --clientset-name=clientset
