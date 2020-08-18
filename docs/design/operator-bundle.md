# Operator Bundle

An `Operator Bundle` is a container image that stores Kubernetes manifests and metadata associated with an operator. A bundle is meant to present a *specific* version of an operator.

## Operator Bundle Overview

The operator manifests refers to a set of Kubernetes manifest(s) the defines the deployment and RBAC model of the operator. The operator metadata on the other hand are, but not limited to:
* Information that identifies the operator, its name, version etc.
* Additional information that drives the UI:
    * Icon
    * Example CR(s)
* Channel(s)
* API(s) provided and required.
* Related images.

An `Operator Bundle` is built as a scratch (non-runnable) container image that contains operator manifests and specific metadata in designated directories inside the image. Then, it can be pushed and pulled from an OCI-compliant container registry. Ultimately, an operator bundle will be used by [Operator Registry](https://github.com/operator-framework/operator-registry) and [Operator-Lifecycle-Manager (OLM)](https://github.com/operator-framework/operator-lifecycle-manager) to install an operator in OLM-enabled clusters.

### Bundle Manifest Format

The standard bundle format requires two directories named `manifests` and `metadata`. The `manifests` directory is where all operator manifests are resided including ClusterServiceVersion (CSV), CustomResourceDefinition (CRD) and other supported Kubernetes types. The `metadata` directory is where operator metadata is located including `annotations.yaml` which contains additional information such as package name, channels and media type. Also, `dependencies.yaml` which contains the operator dependency information can be included in `metadata` directory.

Below is the directory layout of an operator bundle inside a bundle image:
```bash
$ tree
/
├── manifests
│   ├── etcdcluster.crd.yaml
│   └── etcdoperator.clusterserviceversion.yaml
└── metadata
    ├── annotations.yaml
    └── dependencies.yaml
```

*Notes:*
* The names of manifests and metadata directories must match the bundle annotations that are provided in `annotations.yaml` file. Currently, those names are set to `manifests` and `metadata`.

### Bundle Annotations

We use the following labels to annotate the operator bundle image.
* The label `operators.operatorframework.io.bundle.mediatype.v1` reflects the media type or format of the operator bundle. It could be helm charts, plain Kubernetes manifests etc.
* The label `operators.operatorframework.io.bundle.manifests.v1 `reflects the path in the image to the directory that contains the operator manifests. This label is reserved for the future use and is set to `manifests/` for the time being.
* The label `operators.operatorframework.io.bundle.metadata.v1` reflects the path in the image to the directory that contains metadata files about the bundle. This label is reserved for the future use and is set to `metadata/` for the time being.
* The `manifests.v1` and `metadata.v1` labels imply the bundle type:
    * The value `manifests.v1` implies that this bundle contains operator manifests.
    * The value `metadata.v1` implies that this bundle has operator metadata.
* The label `operators.operatorframework.io.bundle.package.v1` reflects the package name of the bundle.
* The label `operators.operatorframework.io.bundle.channels.v1` reflects the list of channels the bundle is subscribing to when added into an operator registry
* The label `operators.operatorframework.io.bundle.channel.default.v1` reflects the default channel an operator should be subscribed to when installed from a registry

The labels will also be put inside a YAML file, as shown below.

*annotations.yaml*
```yaml
annotations:
  operators.operatorframework.io.bundle.mediatype.v1: "registry+v1"
  operators.operatorframework.io.bundle.manifests.v1: "manifests/"
  operators.operatorframework.io.bundle.metadata.v1: "metadata/"
  operators.operatorframework.io.bundle.package.v1: "test-operator"
  operators.operatorframework.io.bundle.channels.v1: "beta,stable"
  operators.operatorframework.io.bundle.channel.default.v1: "stable"
```

*Notes:*
* In case of a mismatch, the `annotations.yaml` file is authoritative because the on-cluster operator-registry that relies on these annotations has access to the yaml file only.
* The potential use case for the `LABELS` is - an external off-cluster tool can inspect the image to check the type of a given bundle image without downloading the content.
* The annotations for bundle manifests and metadata are reserved for future use. They are set to be `manifests/` and `metadata/` for the time being.

### Bundle Dependencies

The dependencies of an operator are listed as a list in `dependencies.yaml` file inside `/metadata` folder of a bundle. This file is optional and only used to specify explicit operator version dependencies at first. Eventually, operator authors can migrate the API-based dependencies into `dependencies.yaml` as well in the future. The ultimate goal is to have `dependencies.yaml` as a centralized metadata for operator dependencies and moving the dependency information away from CSV.

The dependency list will contain a `type` field for each item to specify what kind of dependency this is. There are two supported `type` of operator dependencies. It can be a package type (`olm.package`) meaning this is a dependency for a specific operator version. For `olm.package` type, the dependency information should include the `package` name and the `version` of the package in semver format. We use `blang/semver` library for semver parsing (https://github.com/blang/semver). For example, you can specify an exact version such as `0.5.2` or a range of version such as `>0.5.1` (https://github.com/blang/semver#ranges). In addition, the author can specify dependency that is similar to existing CRD/API-based using `olm.gvk` type and then specify GVK information as how it is done in CSV. This is a path to enable operator authors to consolidate all dependencies (API or explicit version) to be in the same place.

An example of a `dependencies.yaml` that specifies Prometheus operator and etcd CRD dependencies:

```
dependencies:
  - type: olm.package
    value:
      packageName: prometheus
      version: ">0.27.0"
  - type: olm.gvk
    value:
      group: etcd.database.coreos.com
      kind: EtcdCluster
      version: v1beta2
```

### Bundle Dockerfile

This is an example of a `Dockerfile` for operator bundle:
```
FROM scratch

# We are pushing an operator-registry bundle
# that has both metadata and manifests.
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=test-operator
LABEL operators.operatorframework.io.bundle.channels.v1=beta,stable
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable

ADD test/*.yaml /manifests
ADD test/metadata/annotations.yaml /metadata/annotations.yaml
```

## Operator Bundle Commands

`opm` (Operator Package Manager) is a CLI tool to generate bundle annotations, build bundle manifests image, validate bundle manifests image and other functionalities. Please note that the `generate`, `build` and `validate` features of `opm` CLI are currently in alpha and only meant for development use.

### `opm` (Operator Package Manager)

In order to use `opm` CLI, follow the `opm` build instruction:

1. Clone the operator registry repository:

```bash
$ git clone https://github.com/operator-framework/operator-registry
```

2. Build `opm` binary using this command:

```bash
$ make build
```

Now, a binary named `opm` is now built in current directory and ready to be used.

### Generate Bundle Annotations and DockerFile
*Notes:*
* If there are `annotations.yaml` and `Dockerfile` existing in the directory, they will be overwritten.

Using `opm` CLI, bundle annotations can be generated from provided operator manifests. The overall `bundle generate` command usage is:
```bash
Usage:
  opm alpha bundle generate [flags]

Flags:
  -c, --channels string    The list of channels that bundle image belongs to
  -e, --default string     The default channel for the bundle image
  -d, --directory string   The directory where bundle manifests for a specific version are located.
  -h, --help               help for generate
  -u, --output-dir string  Optional output directory for operator manifests
  -p, --package string     The name of the package that bundle image belongs to

  Note:
  * All manifests yaml must be in the same directory.
```

The `--directory/-d`, `--channels/-c`, `--package/-p` are required flags while `--default/-e` and `--output-dir/-u` are optional.

The command for `generate` task is:
```bash
$ ./opm alpha bundle generate --directory /test --package test-operator \
--channels stable,beta --default stable
```

The `--directory` or `-d` specifies the directory where the operator manifests, including CSVs and CRDs, are located. For example:
```bash
$ tree test
test
├── etcdcluster.crd.yaml
└── etcdoperator.clusterserviceversion.yaml
```

The `--package` or `-p` is the name of package fo the operator such as `etcd` which which map `channels` to a particular application definition. `channels` allow package authors to write different upgrade paths for different users (e.g. `beta` vs. `stable`). The `channels` list is provided via `--channels` or `-c` flag. Multiple `channels` are separated by a comma (`,`). The default channel is provided optionally via `--default` or `-e` flag. If the default channel is not provided, the first channel in channel list is selected as default.

All information in `annotations.yaml` is also existed in `LABEL` section of `Dockerfile`.

After the generate command is executed, the `Dockerfile` is generated in the directory where command is run. By default, the `annotations.yaml` file is located in a folder named `metadata` in the same root directory as the input directory containing manifests. For example:
```bash
$ tree test
test
├── my-manifests
│   ├── etcdcluster.crd.yaml
│   └── etcdoperator.clusterserviceversion.yaml
├── metadata
│   └── annotations.yaml
└── Dockerfile
```

If the `--output-dir` parameter is specified, that directory becomes the parent for a new pair of folders `manifests/` and `metadata/`, where `manifests/` is a copy of the passed in directory of manifests and `metadata/` is the folder containing annotations.yaml:

```bash
$ tree test
test
├── my-manifests
│   ├── etcdcluster.crd.yaml
│   └── etcdoperator.clusterserviceversion.yaml
├── my-output-manifest-dir
│   ├── manifests
│   │   ├── etcdoperator.clusterserviceversion.yaml
│   │   └── etcdoperator.clusterserviceversion.yaml
│   └── metadata
│       └── annotations.yaml
└── Dockerfile
```

The `Dockerfile` can be used manually to build the bundle image using container image tools such as Docker, Podman or Buildah. For example, the Docker build command would be:

```bash
$ docker build -f /path/to/Dockerfile -t quay.io/test/test-operator:latest /path/to/manifests/
```

### Build Bundle Image

Operator bundle image can be built from provided operator manifests using `build` command (see *Notes* below). The overall `bundle build` command usage is:
```bash
Usage:
  opm alpha bundle build [flags]

Flags:
  -c, --channels string        The list of channels that bundle image belongs to
  -e, --default string         The default channel for the bundle image
  -d, --directory string       The directory where bundle manifests for a specific version are located
  -h, --help                   help for build
  -b, --image-builder string   Tool to build container images. One of: [docker, podman, buildah] (default "docker")
  -u, --output-dir string      Optional output directory for operator manifests
  -0, --overwrite              To overwrite annotations.yaml if existing
  -p, --package string         The name of the package that bundle image belongs to
  -t, --tag string             The name of the bundle image will be built

  Note:
  * Bundle image is not runnable.
  * All manifests yaml must be in the same directory.
```

The command for `build` task is:
```bash
$ ./opm alpha bundle build --directory /test --tag quay.io/coreos/test-operator.v0.1.0:latest \
--package test-operator --channels stable,beta --default stable
```

The `--directory` or `-d` specifies the directory where the operator manifests for a specific version are located. The `--tag` or `-t` specifies the image tag that you want the operator bundle image to have. By using `build` command, the `annotations.yaml` and `Dockerfile` are automatically generated in the background.

The default image builder is `Docker`. However, ` Buildah` and `Podman` are also supported. An image builder can be specified via `--image-builder` or `-b` optional tag in `build` command. For example:
```bash
$ ./opm alpha bundle build --directory /test/0.1.0/ --tag quay.io/coreos/test-operator.v0.1.0:latest \
--image-builder podman --package test-operator --channels stable,beta --default stable
```

The `--package` or `-p` is the name of package for the operator such as `etcd` which maps `channels` to a particular application definition. `channels` allow package authors to write different upgrade paths for different users (e.g. `beta` vs. `stable`). The `channels` list is provided via `--channels` or `-c` flag. Multiple `channels` are separated by a comma (`,`). The default channel is provided optionally via `--default` or `-e` flag. If the default channel is not provided, the first channel in channel list is selected as default.

*Notes:*
* If there is `Dockerfile` existing in the directory, it will be overwritten.
* If there is an existing `annotations.yaml` in `/metadata` directory, the cli will attempt to validate it and returns any found errors. If the ``annotations.yaml`` is valid, it will be used as a part of build process. The optional boolean `--overwrite/-o` flag can be enabled (false by default) to allow cli to overwrite the `annotations.yaml` if existed.

### Validate Bundle Image

Operator bundle image can validate bundle image that is publicly available in an image registry using `validate` command (see *Notes* below). The overall `bundle validate` command usage is:
```bash
Usage:
  opm alpha bundle validate [flags]

Flags:
  -t, --tag string             The name of the bundle image will be built
  -b, --image-builder string   Tool to extract container images. One of: [docker, podman] (default "docker")
  -h, --help                   help for build
```

The command for `validate` task is:
```bash
$ ./opm alpha bundle validate --tag quay.io/coreos/test-operator.v0.1.0:latest --image-builder docker
```

The `validate` command will first extract the content of the bundle image into a temporary directory after it pulls the image from its image registry. Then, it will validate the format of bundle image to ensure manifests and metadata are located in their appropriate directories (`/manifests/` for bundle manifests files such as CSV and `/metadata/` for metadata files such as `annotations.yaml`). Also, it will validate the information in `annotations.yaml` to confirm that metadata is matching the provided data. For example, the provided media type in annotations.yaml just matches the actual media type is provided in the bundle image.

After the bundle image format is confirmed, the command will validate the bundle contents such as manifests and metadata files if the bundle format is `RegistryV1` or "Plain" type. "RegistryV1" format means it contains `ClusterResourceVersion` and its associated Kubernetes objects while `PlainType` means it contains all Kubernetes objects. The content validation process will ensure the individual file in the bundle image is valid and can be applied to an OLM-enabled cluster provided all necessary permissions and configurations are met.

*Notes:*
* The bundle content validation is best effort which means it will not guarantee 100% accuracy due to nature of Kubernetes objects may need certain permissions and configurations, which users may not have, in order to be applied successfully in a cluster.
