# Releases

## opm

Releases of opm are built by travis, see the [travis.yml](../../.travis.yml) for details.

## Triggering a release

Releases are triggered via tags. Make a new release by tagging a commit with an appropriate semver tag.

## Checking the build

Builds for a release can be found [on travis](https://travis-ci.com/operator-framework/operator-registry). After triggering a build, watch for logs. If the build is successful, a new [release](https://github.com/operator-framework/operator-registry) should appear in GitHub.

## Docker images

Builds are also triggered for the following docker images. The tags in Quay.io will match the git tag:

 - [quay.io/operator-framework/operator-registry-server](https://quay.io/repository/operator-framework/operator-registry-server)
 - [quay.io/operator-framework/configmap-operator-registry](https://quay.io/repository/operator-framework/configmap-operator-registry)
 - [quay.io/operator-framework/upstream-registry-builder](https://quay.io/repository/operator-framework/upstream-registry-builder?tab=tags)
 
 Images are also built to track master with `latest` tags. It is recommended that you always pull by digest, and only use images that are tagged with a version.