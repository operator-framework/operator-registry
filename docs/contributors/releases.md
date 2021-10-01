# Releases

## opm

Binary releases of opm are built by Github Actions, see [release.yaml](../../.github/workflows/release.yaml) for details.
amd64 builds are produced for linux, macos, and windows. 

opm follows semantic versioning, with the latest version derived from the newest semver tag.

## Triggering a release

Releases are triggered via tags. Make a new release by tagging a commit with an appropriate semver tag.

```console
$ git tag -a -m "operator-registry vX.Y.Z" vX.Y.Z
$ git push upstream vX.Y.Z
```

## Checking the build

Builds for a release can be found [on GitHub Actions](https://github.com/operator-framework/operator-registry/actions). After triggering a build, watch for logs. If the build is successful, a new [release](https://github.com/operator-framework/operator-registry/releases) should appear in GitHub. It will be a draft release, so once all the artifacts are available you need to edit the release to publish the draft.

## Docker images

The primary image produced from this repository is [quay.io/operator-framework/opm](https://quay.io/repository/operator-framework/opm). See [goreleaser.yaml](../../.github/workflows/goreleaser.yaml) for details. The following tagging system is used for this image:
 - `:master` - tracks this repository's `master` branch.
 - `:latest` - tracks the highest semver tag in the repository.
 - `vX` - tracks the highest semver tag with major version `X`.
 - `vX.Y` - tracks the highest semver tag with the major/minor version `X.Y`.
 - `vX.Y.Z` - pushed on every non-prerelease semver tag.

For each of the appropriate tags, the build configuration produces images and a manifest list for the following platforms:
 - `linux/amd64`
 - `linux/arm64`
 - `linux/s390x`
 - `linux/ppc64le`

Other deprecated images are also built via Quay triggers. The tags in Quay.io will match the git tag, and for these images, `:latest` tracks the `master` branch:

 - [quay.io/operator-framework/operator-registry-server](https://quay.io/repository/operator-framework/operator-registry-server)
 - [quay.io/operator-framework/configmap-operator-registry](https://quay.io/repository/operator-framework/configmap-operator-registry)
 - [quay.io/operator-framework/upstream-registry-builder](https://quay.io/repository/operator-framework/upstream-registry-builder?tab=tags)
 
It is recommended that you always pull by digest, and only use images that are tagged with a version.

