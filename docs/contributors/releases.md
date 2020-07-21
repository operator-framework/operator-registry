# Releases

## opm

Releases of opm are built by travis, see the [travis.yml](../../.travis.yml) for details.

opm follows semantic versioning, with the latest version derived from the newest semver tag.

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
 
 
## Generating Travis API keys

This requires the travis CLI tool to be installed.

First, backup the existing deploy config in `.travis.yml` - the prompts will overwrite some of the config, and 
we only need the generated api token. This can be done by renaming the `deploy` yaml key to something else.

```sh
$ travis setup releases -r operator-framework/operator-registry --force --pro
```

When prompted, enter credentials for the of-deploy-robot account. Copy the api key from the newly generated `deploy` section in .travis.yml, place
it in the right place in the actual deploy config, and delete the generated deploy section.

You mean need to `travis login --pro` first.
