#!/bin/bash
set -euxo pipefail

echo "Setting Variables"
read -p "Bunlde image: " bundleImage
read -p "Index image tag: " indexImage
read -p "docker or podman: " containerTool

echo "building opm"
go build -mod=vendor  -o bin/opm ./cmd/opm
echo "building prometheus bundles"
./bin/opm alpha bundle build --directory manifests/prometheus/0.14.0 --tag $bundleImage:0.14.0 --package prometheus --channels preview --default preview
./bin/opm alpha bundle build --directory manifests/prometheus/0.15.0 --tag $bundleImage:0.15.0 --package prometheus --channels preview --default preview
./bin/opm alpha bundle build --directory manifests/prometheus/0.22.2 --tag $bundleImage:0.22.2 --package prometheus --channels preview --default preview
echo "pushing prometheus bundles"
$containerTool push $bundleImage:0.14.0
$containerTool push $bundleImage:0.15.0
$containerTool push $bundleImage:0.22.2
echo "building index image with prometheus"
./bin/opm index add -b="$bundleImage:0.14.0,$bundleImage:0.15.0,$bundleImage:0.22.2" -t "$indexImage" -c="$containerTool"
echo "pushing index image"
$containerTool push $indexImage
echo "sleep for 30s before pulling image"
sleep 30s
echo "exporting from index"
./bin/opm index export -i="$indexImage" -o="prometheus" -c="$containerTool"
