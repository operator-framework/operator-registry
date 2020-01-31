#!/bin/bash

username=$2
token=$3
echo "logging in to quay"
docker login -u "$username" -p "$token" quay.io

echo "Generating Random Tag"
bundleTag1=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 6 | head -n 1)
bundleTag2=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 6 | head -n 1)
bundleTag3=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 6 | head -n 1)
indexTag=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 6 | head -n 1)

bundleImage="quay.io/olmtest/e2e-bundle"
indexImage="quay.io/olmtest/e2e-index:$indexTag"

if [ $# -eq 0 ]; then
    echo "No container tool argument supplied"
    exit 1
fi

if [ "$1" != "docker" && "$1" != "podman" ]; then
    echo "container tool argument must be podman or docker"
    exit 1
fi
    
containerTool="$1"
echo "Running with container tool $containerTool"

echo "building opm"
go build -mod=vendor  -o bin/opm ./cmd/opm
echo "building initializer"
go build -mod=vendor  -o bin/initializer ./cmd/initializer

echo "building prometheus bundles"
./bin/opm alpha bundle build --directory manifests/prometheus/0.14.0 --tag $bundleImage:$bundleTag1 --package prometheus --channels preview --default preview
./bin/opm alpha bundle build --directory manifests/prometheus/0.15.0 --tag $bundleImage:$bundleTag2 --package prometheus --channels preview --default preview
./bin/opm alpha bundle build --directory manifests/prometheus/0.22.2 --tag $bundleImage:$bundleTag3 --package prometheus --channels preview --default preview

echo "pushing prometheus bundles"
$containerTool push $bundleImage:$bundleTag1
$containerTool push $bundleImage:$bundleTag2
$containerTool push $bundleImage:$bundleTag3

echo "building index image with prometheus"
./bin/opm index add -b="$bundleImage:$bundleTag1,$bundleImage:$bundleTag2,$bundleImage:$bundleTag3" -t "$indexImage" -c="$containerTool"

echo "pushing index image"
$containerTool push $indexImage

echo "sleep for 30s before pulling image"
sleep 30s

echo "exporting from index"
./bin/opm index export -i="$indexImage" -o="prometheus" -c="$containerTool"

echo "running the initializer on the exported content"
./bin/initializer -m="downloaded"
