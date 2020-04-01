# Daemonless Notes

## Goal

Drive the following stories:

- [OLM-1666](https://issues.redhat.com/browse/OLM-1666)
- [OLM-1679](https://issues.redhat.com/browse/OLM-167://issues.redhat.com/browse/OLM-1679)

TL;DR, we want a supportable way to pull and unpack container images:

- in an unprivileged containerized environment (CI constraint)
- without relying on a external tool (docker, podman, buildah, etc.)

## How does oc unpack images?

The `oc` client needs to pull and unpack images. So we thought it would be useful to see how it achieves this and if we can make use of it.

A quick check of the [extract command](https://github.com/openshift/oc/blob/master/pkg/cli/image/extract/extract.go) reveals several imports relating to images:

```go
 import (
    //...
	"github.com/docker/distribution"
	dockerarchive "github.com/docker/docker/pkg/archive"
	digest "github.com/opencontainers/go-digest"
    //...
	"github.com/openshift/library-go/pkg/image/dockerv1client"
	"github.com/openshift/library-go/pkg/image/registryclient"
	"github.com/openshift/oc/pkg/cli/image/archive"
	"github.com/openshift/oc/pkg/cli/image/imagesource"
	imagemanifest "github.com/openshift/oc/pkg/cli/image/manifest"
	"github.com/openshift/oc/pkg/cli/image/workqueue"
)
```

Right off the bat we can see some direct use of `docker` packages.

Also interesting are a set of private [utility functions for modifying tar headers](https://github.com/openshift/oc/blob/e97def4832ab6489a4237633652ac049f09e685b/pkg/cli/image/extract/extract.go#L554) as well as some for [pulling, walking, and unpacking image layers](https://github.com/openshift/oc/blob/e97def4832ab6489a4237633652ac049f09e685b/pkg/cli/image/extract/extract.go#L326).

The `opencontainers` module is imported for go-digest type.

There are also some OpenShift-specific library packages pulled in. The `openshift/library-go` module provides clients for docker and OpenShift registries as well as some helper types for expressing image manifests for each registry. It also contains various types and utilities for parsing image references.

Some layer unpacking functions [are copied directly from docker](https://github.com/openshift/oc/blob/master/pkg/cli/image/archive/archive.go#L82).

`oc` does support [referencing images from non-registry sources](https://github.com/openshift/oc/blob/e97def4832ab6489a4237633652ac049f09e685b/pkg/cli/image/imagesource/options.go#L22) such as `file` and `s3`, but it's questionable whether we need this in `opm`.

`containers/image`, an upstream module for manipulating images, is used in [image signature verification](https://github.com/openshift/oc/blob/9fd38891f0/pkg/cli/admin/verifyimagesignature/verify-signature.go), though I can't find any indication it's being used for signing in `oc`.

## Options

In lieu of importing `oc` or `openshift/library-go` as a dependency, we took a look at several upstream options.

### Containers Packages

- Serious RedHat presence
- Can be quickly patched by involded RedHatters
- Interfaces feel fine-grained, require glue

### Buildah Packages

[Buildah](https://github.com/containers/buildah) builds on the core `containers` modules. Pulling seems to require some special `chmod`, and may require more investigation.

### Containerd Packages

- No serious RedHat presence
- May need staging repo or in-tree patches for CVEs
- Implements CRI (https://kubernetes.io/docs/setup/production-environment/container-runtimes/#containerd)
- Widespread use/adoption
- Relatively simple interfaces
- Already implemented and working in an [opm branch](https://github.com/njhale/operator-registry/blob/daemonless/pkg/image/containerdregistry/registry.go)

## CRI-O

[CRI-O](https://github.com/cri-o/cri-o/blob/master/server/image_pull.go) needs to pull images and set up container filesystems as well. We haven't looked at this closely yet.
