# Releases

## opm

Releases of opm are built by Github Actions, see the [release.yml](../../.github/workflows/release.yml) for details.
amd64 builds are produced for linux, macos, and windows. 

opm follows semantic versioning, with the latest version derived from the newest semver tag.

## Triggering a release

Releases are triggered via tags. Make a new release by tagging a commit with an appropriate semver tag.

## Checking the build

Builds for a release can be found [on GitHub Actions](https://github.com/operator-framework/operator-registry/actions). After triggering a build, watch for logs. If the build is successful, a new [release](https://github.com/operator-framework/operator-registry/releases) should appear in GitHub. It will be a draft release, so once all the artifacts are available you need to edit the release to publish the draft.

## Docker images

Builds are also triggered for the following docker images. The tags in Quay.io will match the git tag:

 - [quay.io/operator-framework/operator-registry-server](https://quay.io/repository/operator-framework/operator-registry-server)
 - [quay.io/operator-framework/configmap-operator-registry](https://quay.io/repository/operator-framework/configmap-operator-registry)
 - [quay.io/operator-framework/upstream-registry-builder](https://quay.io/repository/operator-framework/upstream-registry-builder?tab=tags)
 
 Images are also built to track master with `latest` tags. It is recommended that you always pull by digest, and only use images that are tagged with a version.
 
