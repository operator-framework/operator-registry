#!/usr/bin/env bash
set -o pipefail

opm='../bin/opm'
db='./test-registry.db'

echo 'testing bundle commutativity'
echo 'adding 0.14.0 prometheus bundle to new index db'
$opm registry add -b "quay.io/operator-framework/operator-bundle-prometheus:0.14.0" -d "test-registry.db" -c docker
echo 'adding 0.22.2 and 0.15.0 versions - this will fail currently'
$opm registry add -b "quay.io/operator-framework/operator-bundle-prometheus:0.22.2","quay.io/operator-framework/operator-bundle-prometheus:0.15.0" -d "test-registry.db" -c docker
rm -f $db
